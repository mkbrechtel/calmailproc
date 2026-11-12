//! Command line: flags, configuration and choosing the mode.
//!
//! Settings come from the flags first, then from the config file. Flags use
//! the Go `flag` syntax of calmailproc 0.1 (`-url x`, `--url x`, `-url=x`),
//! so existing mail filter recipes keep working.

use std::process::{Command, ExitCode, Stdio};

use crate::config::{self, Config};
use crate::error::{Context, Error, Result};
use crate::processor::{Processor, maildir, stdin};
use crate::storage::{CalDavStorage, WebdavConfig};

pub const USAGE: &str = "\
Usage of calmailproc:
  -calendar string
        CalDAV calendar path (e.g., /calendar/)
  -delete-cancelled
        Delete cancelled events and occurrences instead of marking them
  -drop-alarms
        Remove alarms from events before storing them
  -ignore-unknown-cancellations
        Ignore a cancellation if its event is not stored
  -maildir string
        Path to maildir to process (will process all emails recursively)
  -pass string
        CalDAV password
  -pass-cmd string
        Shell command that prints the CalDAV password (instead of -pass)
  -process-replies
        Process attendance replies to update events
  -process-replies-without-event
        Store a reply as an event if its event is not stored
  -url string
        CalDAV server URL (e.g., http://localhost:5232)
  -user string
        CalDAV username
  -verbose
        Enable verbose logging output
  -version
        Print the version and exit

Without -maildir, a single email is read from standard input.
Settings not given as flags are read from
$XDG_CONFIG_HOME/calmailproc/config.yaml (default ~/.config).";

/// The flags that were given on the command line.
#[derive(Debug, Default, PartialEq, Eq)]
pub struct Flags {
    pub url: Option<String>,
    pub user: Option<String>,
    pub pass: Option<String>,
    pub pass_cmd: Option<String>,
    pub calendar: Option<String>,
    pub maildir: Option<String>,
    pub process_replies: Option<bool>,
    pub process_replies_without_event: Option<bool>,
    pub ignore_unknown_cancellations: Option<bool>,
    pub delete_cancelled: Option<bool>,
    pub drop_alarms: Option<bool>,
    pub verbose: Option<bool>,
    pub help: bool,
    pub version: bool,
}

/// Parses the arguments after the program name.
pub fn parse_flags(args: impl IntoIterator<Item = String>) -> Result<Flags> {
    let mut flags = Flags::default();
    let mut args = args.into_iter();

    while let Some(arg) = args.next() {
        // Like Go's flag package: parsing stops at "--" or the first
        // argument that is not a flag.
        let Some(flag) = arg.strip_prefix("--").or_else(|| arg.strip_prefix('-')) else {
            break;
        };
        if flag.is_empty() {
            break;
        }
        let (name, inline_value) = match flag.split_once('=') {
            Some((name, value)) => (name, Some(value.to_string())),
            None => (flag, None),
        };

        let mut string_value = |name: &str| -> Result<Option<String>> {
            inline_value
                .clone()
                .or_else(|| args.next())
                .map(Some)
                .ok_or_else(|| Error::new(format!("flag needs an argument: -{name}")))
        };
        let bool_value = |name: &str| -> Result<Option<bool>> {
            match inline_value.as_deref() {
                None | Some("true" | "1" | "t" | "T" | "TRUE" | "True") => Ok(Some(true)),
                Some("false" | "0" | "f" | "F" | "FALSE" | "False") => Ok(Some(false)),
                Some(other) => Err(Error::new(format!(
                    "invalid boolean value {other:?} for -{name}"
                ))),
            }
        };

        match name {
            "url" => flags.url = string_value(name)?,
            "user" => flags.user = string_value(name)?,
            "pass" => flags.pass = string_value(name)?,
            "pass-cmd" => flags.pass_cmd = string_value(name)?,
            "calendar" => flags.calendar = string_value(name)?,
            "maildir" => flags.maildir = string_value(name)?,
            "process-replies" => flags.process_replies = bool_value(name)?,
            "process-replies-without-event" => {
                flags.process_replies_without_event = bool_value(name)?
            }
            "ignore-unknown-cancellations" => {
                flags.ignore_unknown_cancellations = bool_value(name)?
            }
            "delete-cancelled" => flags.delete_cancelled = bool_value(name)?,
            "drop-alarms" => flags.drop_alarms = bool_value(name)?,
            "verbose" => flags.verbose = bool_value(name)?,
            "h" | "help" => flags.help = true,
            "version" => flags.version = true,
            _ => {
                return Err(Error::new(format!(
                    "flag provided but not defined: -{name}"
                )));
            }
        }
    }
    Ok(flags)
}

/// Applies the flags on top of the config file settings.
pub fn merge(mut config: Config, flags: Flags) -> Config {
    let set = |target: &mut String, value: Option<String>| {
        if let Some(value) = value.filter(|v| !v.is_empty()) {
            *target = value;
        }
    };
    // A password source given as a flag replaces whichever one the config
    // file has, so `-pass` overrides `pass_cmd` and `-pass-cmd` overrides `pass`.
    let given = |flag: &Option<String>| flag.as_deref().is_some_and(|value| !value.is_empty());
    if given(&flags.pass) && !given(&flags.pass_cmd) {
        config.webdav.pass_cmd.clear();
    }
    if given(&flags.pass_cmd) && !given(&flags.pass) {
        config.webdav.pass.clear();
    }
    set(&mut config.webdav.url, flags.url);
    set(&mut config.webdav.user, flags.user);
    set(&mut config.webdav.pass, flags.pass);
    set(&mut config.webdav.pass_cmd, flags.pass_cmd);
    set(&mut config.webdav.calendar, flags.calendar);
    set(&mut config.maildir.path, flags.maildir);
    let processor = &mut config.processor;
    for (target, value) in [
        (&mut processor.process_replies, flags.process_replies),
        (
            &mut processor.process_replies_without_event,
            flags.process_replies_without_event,
        ),
        (
            &mut processor.ignore_unknown_cancellations,
            flags.ignore_unknown_cancellations,
        ),
        (&mut processor.delete_cancelled, flags.delete_cancelled),
        (&mut processor.drop_alarms, flags.drop_alarms),
        (&mut config.maildir.verbose, flags.verbose),
    ] {
        if let Some(value) = value {
            *target = value;
        }
    }
    config
}

/// Processes the maildir if one is configured, standard input otherwise.
pub fn run(config: &Config) -> Result<()> {
    let webdav = &config.webdav;
    if [&webdav.url, &webdav.user, &webdav.calendar]
        .iter()
        .any(|value| value.is_empty())
        || (webdav.pass.is_empty() && webdav.pass_cmd.is_empty())
    {
        return Err(Error::new(
            "all CalDAV flags are required: -url, -user, -pass (or -pass-cmd), -calendar",
        ));
    }

    let webdav = WebdavConfig {
        pass: resolve_password(webdav)?,
        ..webdav.clone()
    };
    let storage = CalDavStorage::from_config(&webdav);
    let mut processor = Processor::from_config(storage, &config.processor);

    if !config.maildir.path.is_empty() {
        return maildir::process_with_config(&config.maildir, &mut processor)
            .context("error processing maildir");
    }
    stdin::process(&mut processor).context("error processing stdin")
}

/// The password: `pass`, or the output of `pass_cmd`.
///
/// `pass_cmd` works like mbsync's `PassCmd`: the command is run by the shell
/// and must print exactly one line, the password. This keeps the password out
/// of the config file, for example with `pass show ...` or `secret-tool`.
pub fn resolve_password(webdav: &WebdavConfig) -> Result<String> {
    if webdav.pass_cmd.is_empty() {
        return Ok(webdav.pass.clone());
    }
    if !webdav.pass.is_empty() {
        return Err(Error::new(
            "both a password and a password command are set; use only one of pass and pass_cmd",
        ));
    }

    let (shell, flag) = if cfg!(windows) {
        ("cmd", "/C")
    } else {
        ("sh", "-c")
    };
    let output = Command::new(shell)
        .arg(flag)
        .arg(&webdav.pass_cmd)
        // Standard input carries the mail in stdin mode; the command must not
        // read it. A password prompt uses the terminal, not standard input.
        .stdin(Stdio::null())
        .stderr(Stdio::inherit())
        .output()
        .context("running the password command")?;
    if !output.status.success() {
        return Err(Error::new(format!(
            "the password command failed ({})",
            output.status
        )));
    }
    let text = String::from_utf8(output.stdout)
        .map_err(|_| Error::new("the password command printed something that is not UTF-8"))?;
    let password = text
        .strip_suffix('\n')
        .map(|line| line.strip_suffix('\r').unwrap_or(line))
        .unwrap_or(&text);
    if password.is_empty() {
        return Err(Error::new("the password command printed nothing"));
    }
    if password.contains('\n') {
        return Err(Error::new(
            "the password command printed more than one line; it must print only the password",
        ));
    }
    Ok(password.to_string())
}

/// The whole program: parse flags, load the config, run.
pub fn main() -> ExitCode {
    let flags = match parse_flags(std::env::args().skip(1)) {
        Ok(flags) => flags,
        Err(error) => {
            eprintln!("{error}\n{USAGE}");
            return ExitCode::from(2);
        }
    };
    if flags.help {
        println!("{USAGE}");
        return ExitCode::SUCCESS;
    }
    if flags.version {
        println!("calmailproc {}", env!("CARGO_PKG_VERSION"));
        return ExitCode::SUCCESS;
    }

    let file_config = config::load().unwrap_or_else(|error| {
        eprintln!("Warning: failed to load config file: {error}");
        Config::default()
    });
    match run(&merge(file_config, flags)) {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("Error: {error}");
            ExitCode::FAILURE
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn parse(args: &[&str]) -> Result<Flags> {
        parse_flags(args.iter().map(|s| s.to_string()))
    }

    #[test]
    fn parses_go_style_flags() {
        let flags = parse(&[
            "-process-replies",
            "-url",
            "http://localhost:15232",
            "--user=test",
            "-pass",
            "pass",
            "-calendar=/user/calendars/calendar1/",
            "--maildir",
            "test/maildir",
            "-verbose",
        ])
        .unwrap();
        assert_eq!(flags.url.as_deref(), Some("http://localhost:15232"));
        assert_eq!(flags.user.as_deref(), Some("test"));
        assert_eq!(flags.pass.as_deref(), Some("pass"));
        assert_eq!(
            flags.calendar.as_deref(),
            Some("/user/calendars/calendar1/")
        );
        assert_eq!(flags.maildir.as_deref(), Some("test/maildir"));
        assert_eq!(flags.process_replies, Some(true));
        assert_eq!(flags.verbose, Some(true));
    }

    #[test]
    fn boolean_flags_take_inline_values_only() {
        let flags = parse(&[
            "-verbose=false",
            "-process-replies=true",
            "-process-replies-without-event",
            "--delete-cancelled",
            "-ignore-unknown-cancellations",
            "-drop-alarms=false",
        ])
        .unwrap();
        assert_eq!(flags.verbose, Some(false));
        assert_eq!(flags.process_replies, Some(true));
        assert_eq!(flags.process_replies_without_event, Some(true));
        assert_eq!(flags.delete_cancelled, Some(true));
        assert_eq!(flags.ignore_unknown_cancellations, Some(true));
        assert_eq!(flags.drop_alarms, Some(false));
        assert!(parse(&["-verbose=perhaps"]).is_err());
    }

    #[test]
    fn rejects_unknown_flags_and_missing_values() {
        let error = parse(&["-caldav", "x"]).unwrap_err().to_string();
        assert_eq!(error, "flag provided but not defined: -caldav");
        let error = parse(&["-url"]).unwrap_err().to_string();
        assert_eq!(error, "flag needs an argument: -url");
    }

    #[test]
    fn stops_at_first_non_flag_and_handles_help() {
        let flags = parse(&["-user", "a", "positional", "-pass", "ignored"]).unwrap();
        assert_eq!(flags.user.as_deref(), Some("a"));
        assert_eq!(flags.pass, None);
        assert!(parse(&["-h"]).unwrap().help);
        assert!(parse(&["--help"]).unwrap().help);
        assert!(parse(&["-version"]).unwrap().version);
        assert_eq!(parse(&[]).unwrap(), Flags::default());
    }

    #[test]
    fn flags_override_config_file() {
        let file = config::parse(
            "webdav:\n  url: http://file\n  user: fileuser\n  pass: filepass\n  calendar: /file/\nprocessor:\n  process_replies: true\nmaildir:\n  path: /mail\n  verbose: true\n",
        )
        .unwrap();
        let flags = parse(&[
            "-url",
            "http://flag",
            "-verbose=false",
            "-process-replies-without-event",
        ])
        .unwrap();
        let merged = merge(file, flags);
        assert!(merged.processor.process_replies_without_event);
        assert!(!merged.processor.delete_cancelled);
        assert_eq!(merged.webdav.url, "http://flag");
        assert_eq!(merged.webdav.user, "fileuser");
        assert_eq!(merged.maildir.path, "/mail");
        assert!(merged.processor.process_replies);
        assert!(!merged.maildir.verbose);
    }

    fn with_pass_cmd(command: &str) -> WebdavConfig {
        WebdavConfig {
            pass_cmd: command.to_string(),
            ..WebdavConfig::default()
        }
    }

    #[test]
    #[cfg(unix)]
    fn password_command_prints_the_password() {
        let password = |command: &str| resolve_password(&with_pass_cmd(command));
        assert_eq!(password("echo secret").unwrap(), "secret");
        // Without a trailing newline, with CRLF, and with spaces kept.
        assert_eq!(password("printf 'se cret '").unwrap(), "se cret ");
        assert_eq!(password("printf 'secret\\r\\n'").unwrap(), "secret");
        // The shell runs it, so pipes and variables work.
        assert_eq!(password("echo top-secret | cut -d- -f2").unwrap(), "secret");
        // It does not get our standard input.
        assert_eq!(password("cat; echo secret").unwrap(), "secret");
    }

    #[test]
    #[cfg(unix)]
    fn password_command_failures_are_errors() {
        let error = |command: &str| {
            resolve_password(&with_pass_cmd(command))
                .unwrap_err()
                .to_string()
        };
        assert!(error("exit 3").contains("the password command failed"));
        assert!(error("echo secret; false").contains("the password command failed"));
        assert!(error("true").contains("printed nothing"));
        assert!(error("echo").contains("printed nothing"));
        assert!(error("printf 'a\\nb\\n'").contains("more than one line"));
        // The password itself never appears in an error.
        assert!(!error("printf 'hunter2\\nsecond\\n'").contains("hunter2"));
    }

    #[test]
    fn plain_password_needs_no_command() {
        let config = WebdavConfig {
            pass: "plain".to_string(),
            ..WebdavConfig::default()
        };
        assert_eq!(resolve_password(&config).unwrap(), "plain");

        let both = WebdavConfig {
            pass: "plain".to_string(),
            pass_cmd: "echo other".to_string(),
            ..WebdavConfig::default()
        };
        let error = resolve_password(&both).unwrap_err().to_string();
        assert!(
            error.contains("use only one of pass and pass_cmd"),
            "{error}"
        );
    }

    #[test]
    fn a_password_flag_replaces_the_other_kind_from_the_config_file() {
        let file = || config::parse("webdav:\n  pass_cmd: pass show caldav\n").unwrap();
        assert_eq!(file().webdav.pass_cmd, "pass show caldav");

        let merged = merge(file(), parse(&["-pass", "typed"]).unwrap());
        assert_eq!(merged.webdav.pass, "typed");
        assert_eq!(merged.webdav.pass_cmd, "");

        let file = config::parse("webdav:\n  pass: stored\n").unwrap();
        let merged = merge(file, parse(&["-pass-cmd", "echo x"]).unwrap());
        assert_eq!(merged.webdav.pass, "");
        assert_eq!(merged.webdav.pass_cmd, "echo x");

        // Both as flags is a contradiction and reported when the password is needed.
        let merged = merge(
            Config::default(),
            parse(&["-pass", "a", "-pass-cmd", "echo b"]).unwrap(),
        );
        assert!(resolve_password(&merged.webdav).is_err());
    }

    #[test]
    fn run_requires_caldav_settings() {
        let error = run(&Config::default()).unwrap_err().to_string();
        assert!(error.contains("all CalDAV flags are required"), "{error}");
    }
}
