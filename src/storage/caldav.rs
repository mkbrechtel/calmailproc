//! CalDAV storage.
//!
//! Each event is one resource at `{calendar}/{UID}.ics`, so reading and
//! writing are plain HTTP `GET` and `PUT` with basic authentication. No WebDAV
//! queries are needed.

use std::time::Duration;

use base64::Engine;
use serde::Deserialize;
use ureq::Agent;

use super::Storage;
use crate::error::{Context, Error, Result};
use crate::ical::{self, Event};

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct WebdavConfig {
    pub url: String,
    pub user: String,
    pub pass: String,
    pub calendar: String,
}

pub struct CalDavStorage {
    agent: Agent,
    /// Server URL plus calendar path, always ending in `/`.
    calendar_url: String,
    authorization: String,
}

impl CalDavStorage {
    pub fn from_config(config: &WebdavConfig) -> Self {
        Self::new(&config.url, &config.user, &config.pass, &config.calendar)
    }

    pub fn new(server_url: &str, username: &str, password: &str, calendar_path: &str) -> Self {
        let agent = Agent::new_with_config(
            Agent::config_builder()
                .timeout_global(Some(Duration::from_secs(60)))
                .build(),
        );
        let credentials =
            base64::engine::general_purpose::STANDARD.encode(format!("{username}:{password}"));
        CalDavStorage {
            agent,
            calendar_url: calendar_url(server_url, calendar_path),
            authorization: format!("Basic {credentials}"),
        }
    }

    fn event_url(&self, uid: &str) -> String {
        format!("{}{uid}.ics", self.calendar_url)
    }
}

impl Storage for CalDavStorage {
    fn store_event(&mut self, event: &Event) -> Result<()> {
        if event.uid.is_empty() {
            return Err(Error::new("event has no UID"));
        }
        if event.raw_data.is_empty() {
            return Err(Error::new("no raw calendar data to store"));
        }
        self.agent
            .put(self.event_url(&event.uid))
            .header("Authorization", &self.authorization)
            .content_type("text/calendar; charset=utf-8")
            .send(event.raw_data.as_str())
            .context("storing event via CalDAV")?;
        Ok(())
    }

    fn get_event(&mut self, uid: &str) -> Result<Option<Event>> {
        let response = self
            .agent
            .get(self.event_url(uid))
            .header("Authorization", &self.authorization)
            .call();
        let mut response = match response {
            Ok(response) => response,
            Err(ureq::Error::StatusCode(404)) => return Ok(None),
            Err(error) => return Err(error).context("getting event from CalDAV"),
        };
        let ics = response
            .body_mut()
            .read_to_string()
            .context("reading event from CalDAV")?;
        // Summary and sequence come from the stored data, so an update is
        // compared against what the server really has.
        let event = ical::parse_ical_data(&ics).context("parsing event from CalDAV")?;
        Ok(Some(Event {
            uid: uid.to_string(),
            ..event
        }))
    }
}

/// Joins server URL and calendar path with exactly one `/` between them and
/// one at the end.
fn calendar_url(server_url: &str, calendar_path: &str) -> String {
    let mut url = format!(
        "{}/{}",
        server_url.trim_end_matches('/'),
        calendar_path.trim_start_matches('/')
    );
    if !url.ends_with('/') {
        url.push('/');
    }
    url
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn builds_event_urls() {
        let expected = "http://localhost:5232/user/calendars/cal/";
        assert_eq!(
            calendar_url("http://localhost:5232", "/user/calendars/cal/"),
            expected
        );
        assert_eq!(
            calendar_url("http://localhost:5232/", "user/calendars/cal"),
            expected
        );

        let storage = CalDavStorage::new("http://localhost:5232", "test", "pass", "/cal/");
        assert_eq!(
            storage.event_url("abc@example.com"),
            "http://localhost:5232/cal/abc@example.com.ics"
        );
        // "test:pass" in base64
        assert_eq!(storage.authorization, "Basic dGVzdDpwYXNz");
    }
}
