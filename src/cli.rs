//! Command line: flags, configuration and choosing the mode.
//!
//! Settings come from the flags first, then from the config file. Flags use
//! the Go `flag` syntax of calmailproc 0.1 (`-url x`, `--url x`, `-url=x`),
//! so existing mail filter recipes keep working.

use std::process::ExitCode;

use crate::config::{self, Config};
use crate::error::{Context, Error, Result};
use crate::processor::{Processor, maildir, stdin};
use crate::storage::CalDavStorage;

pub const USAGE: &str = "\
Usage of calmailproc:
  -calendar string
        CalDAV calendar path (e.g., /calendar/)
  -delete-cancelled
        Delete cancelled events and occurrences instead of marking them
  -drop-alarms
        Remove alarms from events before storing them
  -force
        Store a reply even if its event is not stored
  -ignore-unknown-cancellations
        Ignore a cancellation if its event is not stored
  -maildir string
        Path to maildir to process (will process all emails recursively)
  -pass string
        CalDAV password
  -process-replies
        Process attendance replies to update events
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
    pub calendar: Option<String>,
    pub maildir: Option<String>,
    pub process_replies: Option<bool>,
    pub force: Option<bool>,
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
            "calendar" => flags.calendar = string_value(name)?,
            "maildir" => flags.maildir = string_value(name)?,
            "process-replies" => flags.process_replies = bool_value(name)?,
            "force" => flags.force = bool_value(name)?,
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
    set(&mut config.webdav.url, flags.url);
    set(&mut config.webdav.user, flags.user);
    set(&mut config.webdav.pass, flags.pass);
    set(&mut config.webdav.calendar, flags.calendar);
    set(&mut config.maildir.path, flags.maildir);
    let processor = &mut config.processor;
    for (target, value) in [
        (&mut processor.process_replies, flags.process_replies),
        (&mut processor.force, flags.force),
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
    if [&webdav.url, &webdav.user, &webdav.pass, &webdav.calendar]
        .iter()
        .any(|value| value.is_empty())
    {
        return Err(Error::new(
            "all CalDAV flags are required: -url, -user, -pass, -calendar",
        ));
    }

    let storage = CalDavStorage::from_config(webdav);
    let mut processor = Processor::from_config(storage, &config.processor);

    if !config.maildir.path.is_empty() {
        return maildir::process_with_config(&config.maildir, &mut processor)
            .context("error processing maildir");
    }
    stdin::process(&mut processor).context("error processing stdin")
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
            "-force",
            "--delete-cancelled",
            "-ignore-unknown-cancellations",
            "-drop-alarms=false",
        ])
        .unwrap();
        assert_eq!(flags.verbose, Some(false));
        assert_eq!(flags.process_replies, Some(true));
        assert_eq!(flags.force, Some(true));
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
        let flags = parse(&["-url", "http://flag", "-verbose=false", "-force"]).unwrap();
        let merged = merge(file, flags);
        assert!(merged.processor.force);
        assert!(!merged.processor.delete_cancelled);
        assert_eq!(merged.webdav.url, "http://flag");
        assert_eq!(merged.webdav.user, "fileuser");
        assert_eq!(merged.maildir.path, "/mail");
        assert!(merged.processor.process_replies);
        assert!(!merged.maildir.verbose);
    }

    #[test]
    fn run_requires_caldav_settings() {
        let error = run(&Config::default()).unwrap_err().to_string();
        assert!(error.contains("all CalDAV flags are required"), "{error}");
    }
}
