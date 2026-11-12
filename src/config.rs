//! The YAML configuration file at `$XDG_CONFIG_HOME/calmailproc/config.yaml`.

use std::path::PathBuf;

use serde::Deserialize;

use crate::error::{Context, Result};
use crate::processor::ProcessorConfig;
use crate::processor::maildir::MaildirConfig;
use crate::storage::WebdavConfig;

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct Config {
    pub webdav: WebdavConfig,
    pub processor: ProcessorConfig,
    pub maildir: MaildirConfig,
}

/// Loads the config file. A missing file gives the default (empty) config.
pub fn load() -> Result<Config> {
    match config_path() {
        Some(path) if path.exists() => {
            let text = std::fs::read_to_string(&path).context("failed to read config file")?;
            parse(&text)
        }
        _ => Ok(Config::default()),
    }
}

pub fn parse(yaml: &str) -> Result<Config> {
    if yaml.trim().is_empty() {
        return Ok(Config::default());
    }
    serde_saphyr::from_str(yaml).context("failed to parse config file")
}

/// `$XDG_CONFIG_HOME/calmailproc/config.yaml`, with `~/.config` as the
/// default for an unset or relative `XDG_CONFIG_HOME` (XDG base directory spec).
fn config_path() -> Option<PathBuf> {
    let config_home = std::env::var_os("XDG_CONFIG_HOME")
        .map(PathBuf::from)
        .filter(|path| path.is_absolute())
        .or_else(|| std::env::var_os("HOME").map(|home| PathBuf::from(home).join(".config")))?;
    Some(config_home.join("calmailproc").join("config.yaml"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_full_config() {
        let config = parse(
            "webdav:\n  url: http://localhost:15232\n  user: test\n  pass: \"p:a#ss\"\n  calendar: /user/calendars/calendar3/\n\nprocessor:\n  process_replies: true\n  process_replies_without_event: true\n  ignore_unknown_cancellations: true\n  delete_cancelled: true\n  drop_alarms: true\n\nmaildir:\n  path: test/maildir\n  verbose: true\n",
        )
        .unwrap();
        assert!(config.processor.process_replies_without_event);
        assert!(config.processor.ignore_unknown_cancellations);
        assert!(config.processor.delete_cancelled);
        assert!(config.processor.drop_alarms);
        assert_eq!(config.webdav.url, "http://localhost:15232");
        assert_eq!(config.webdav.user, "test");
        assert_eq!(config.webdav.pass, "p:a#ss");
        assert_eq!(config.webdav.pass_cmd, "");
        assert_eq!(config.webdav.calendar, "/user/calendars/calendar3/");
        assert!(config.processor.process_replies);
        assert_eq!(config.maildir.path, "test/maildir");
        assert!(config.maildir.verbose);
    }

    #[test]
    fn missing_sections_and_unknown_keys_are_fine() {
        let config =
            parse("webdav:\n  url: http://example.com\nstdin:\n  enabled: true\n").unwrap();
        assert_eq!(config.webdav.url, "http://example.com");
        assert_eq!(config.webdav.user, "");
        assert!(!config.processor.process_replies);
        assert!(!config.processor.process_replies_without_event);
        assert!(!config.processor.ignore_unknown_cancellations);
        assert!(!config.processor.delete_cancelled);
        assert!(!config.processor.drop_alarms);
        assert_eq!(config.maildir.path, "");
        assert!(parse("").is_ok());
    }

    #[test]
    fn invalid_yaml_is_an_error() {
        assert!(parse("webdav: [unclosed").is_err());
        assert!(parse("processor:\n  process_replies: maybe\n").is_err());
    }

    #[test]
    fn reads_the_test_config_file() {
        let text = std::fs::read_to_string("test/config/calmailproc/config.yaml").unwrap();
        let config = parse(&text).unwrap();
        assert_eq!(config.webdav.calendar, "/user/calendars/calendar3/");
        assert!(config.processor.process_replies);
    }
}
