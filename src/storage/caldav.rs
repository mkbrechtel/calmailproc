//! CalDAV storage.
//!
//! calmailproc stores an event as the resource `{calendar}/{UID}.ics`, but
//! an event that something else put on the server (a calendar client, or the
//! server's own scheduling) can have any name. So an event is looked up by
//! name first and, if that finds nothing, by a `calendar-query` for its UID.
//! It is written back to wherever it was found.
//!
//! Writes are conditional (`If-Match` with the ETag that was read,
//! `If-None-Match: *` for a new event), so a change made by someone else in
//! between is not overwritten.

use std::time::Duration;

use base64::Engine;
use percent_encoding::{AsciiSet, NON_ALPHANUMERIC, utf8_percent_encode};
use serde::Deserialize;
use ureq::http::Request;
use ureq::{Agent, AsSendBody};

use super::{Storage, StoreError, Stored, Version};
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
    /// Scheme and host of the server, for the paths in query results.
    origin: String,
    /// Server URL plus calendar path, always ending in `/`.
    calendar_url: String,
    authorization: String,
}

/// Characters of a UID that are used as they are in a resource name. Every
/// other character is percent-encoded. `@`, `:` and `+` stay, so that the
/// names are the ones earlier versions used.
const NAME_ESCAPES: &AsciiSet = &NON_ALPHANUMERIC
    .remove(b'-')
    .remove(b'_')
    .remove(b'.')
    .remove(b'~')
    .remove(b'@')
    .remove(b':')
    .remove(b'+');

struct Reply {
    status: u16,
    etag: Option<String>,
    body: String,
}

const CALDAV_NAMESPACE: &str = "urn:ietf:params:xml:ns:caldav";

impl Reply {
    /// True if the body is a WebDAV error naming a CalDAV precondition, such
    /// as `valid-calendar-data` or `no-uid-conflict`.
    ///
    /// Servers answer 412 both when `If-Match` fails and when they reject the
    /// calendar data (Xandikos does). Only the first is a conflict that
    /// reading again can resolve.
    fn is_caldav_error(&self) -> bool {
        roxmltree::Document::parse(&self.body).is_ok_and(|document| {
            document.root_element().tag_name().name() == "error"
                && document
                    .root_element()
                    .children()
                    .any(|child| child.tag_name().namespace() == Some(CALDAV_NAMESPACE))
        })
    }

    /// An error naming the status and the server's own explanation, which is
    /// usually the only useful part (for example Open-Xchange's `CAL-4034`).
    fn error(&self, action: &str) -> Error {
        let reason: String = self.body.split_whitespace().collect::<Vec<_>>().join(" ");
        let reason: String = reason.chars().take(300).collect();
        if reason.is_empty() {
            Error::new(format!("{action}: HTTP status {}", self.status))
        } else {
            Error::new(format!("{action}: HTTP status {}: {reason}", self.status))
        }
    }
}

impl CalDavStorage {
    pub fn from_config(config: &WebdavConfig) -> Self {
        Self::new(&config.url, &config.user, &config.pass, &config.calendar)
    }

    pub fn new(server_url: &str, username: &str, password: &str, calendar_path: &str) -> Self {
        let agent = Agent::new_with_config(
            Agent::config_builder()
                .timeout_global(Some(Duration::from_secs(60)))
                // Statuses are handled here, with the response body.
                .http_status_as_error(false)
                // REPORT is a WebDAV method.
                .allow_non_standard_methods(true)
                .build(),
        );
        let credentials =
            base64::engine::general_purpose::STANDARD.encode(format!("{username}:{password}"));
        CalDavStorage {
            agent,
            origin: origin(server_url),
            calendar_url: calendar_url(server_url, calendar_path),
            authorization: format!("Basic {credentials}"),
        }
    }

    fn event_url(&self, uid: &str) -> String {
        format!(
            "{}{}.ics",
            self.calendar_url,
            utf8_percent_encode(uid, NAME_ESCAPES)
        )
    }

    fn send(
        &self,
        method: &str,
        url: &str,
        headers: &[(&str, &str)],
        body: impl AsSendBody,
    ) -> Result<Reply> {
        let mut request = Request::builder()
            .method(method)
            .uri(url)
            .header("Authorization", &self.authorization);
        for (name, value) in headers {
            request = request.header(*name, *value);
        }
        let request = request
            .body(body)
            .map_err(|e| Error::new(format!("building request: {e}")))?;
        let mut response = self
            .agent
            .run(request)
            .map_err(|e| Error::new(e.to_string()))?;
        let etag = response
            .headers()
            .get("ETag")
            .and_then(|value| value.to_str().ok())
            .map(str::to_string);
        Ok(Reply {
            status: response.status().as_u16(),
            etag,
            body: response
                .body_mut()
                .read_to_string()
                .map_err(|e| Error::new(e.to_string()))?,
        })
    }

    /// Looks for an event with this UID under any name.
    fn find_by_uid(&self, uid: &str) -> Result<Option<Stored>> {
        let query = format!(
            r#"<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop><D:getetag/><C:calendar-data/></D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:prop-filter name="UID">
          <C:text-match collation="i;octet">{}</C:text-match>
        </C:prop-filter>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>"#,
            xml_escape(uid)
        );
        let reply = self.send(
            "REPORT",
            &self.calendar_url,
            &[
                ("Depth", "1"),
                ("Content-Type", "application/xml; charset=utf-8"),
            ],
            query,
        )?;
        if reply.status != 207 {
            return Err(reply.error("querying the calendar for the event's UID"));
        }

        for (href, etag, ics) in parse_multistatus(&reply.body)? {
            // The query matches substrings, so check the UID itself.
            let Ok(event) = ical::parse_ical_data(&ics) else {
                continue;
            };
            if event.uid == uid {
                let location = if href.starts_with('/') {
                    format!("{}{href}", self.origin)
                } else {
                    href
                };
                return Ok(Some(Stored {
                    event,
                    version: Version { location, etag },
                }));
            }
        }
        Ok(None)
    }
}

impl Storage for CalDavStorage {
    fn get_event(&mut self, uid: &str) -> Result<Option<Stored>> {
        let url = self.event_url(uid);
        let reply = self
            .send("GET", &url, &[], ())
            .context("getting event from CalDAV")?;
        match reply.status {
            200 => {
                // Summary and sequence come from the stored data, so an update
                // is compared against what the server really has.
                let event =
                    ical::parse_ical_data(&reply.body).context("parsing event from CalDAV")?;
                Ok(Some(Stored {
                    event: Event {
                        uid: uid.to_string(),
                        ..event
                    },
                    version: Version {
                        location: url,
                        etag: reply.etag,
                    },
                }))
            }
            404 => self.find_by_uid(uid).context("getting event from CalDAV"),
            _ => Err(reply.error("getting event from CalDAV")),
        }
    }

    fn store_event(
        &mut self,
        event: &Event,
        replaces: Option<&Version>,
    ) -> std::result::Result<(), StoreError> {
        if event.uid.is_empty() {
            return Err(Error::new("event has no UID").into());
        }
        if event.raw_data.is_empty() {
            return Err(Error::new("no raw calendar data to store").into());
        }
        let new_url = self.event_url(&event.uid);
        let (url, condition) = match replaces {
            Some(Version {
                location,
                etag: Some(etag),
            }) => (location.as_str(), Some(("If-Match", etag.as_str()))),
            // The server gave no ETag when the event was read.
            Some(Version {
                location,
                etag: None,
            }) => (location.as_str(), None),
            None => (new_url.as_str(), Some(("If-None-Match", "*"))),
        };
        let mut headers = vec![("Content-Type", "text/calendar; charset=utf-8")];
        headers.extend(condition);

        let reply = self
            .send("PUT", url, &headers, event.raw_data.as_str())
            .context("storing event via CalDAV")?;
        match reply.status {
            200..=299 => Ok(()),
            412 if !reply.is_caldav_error() => Err(StoreError::Conflict),
            _ => Err(reply.error("storing event via CalDAV").into()),
        }
    }

    fn delete_event(&mut self, version: &Version) -> std::result::Result<(), StoreError> {
        let condition: Vec<(&str, &str)> = version
            .etag
            .iter()
            .map(|etag| ("If-Match", etag.as_str()))
            .collect();
        let reply = self
            .send("DELETE", &version.location, &condition, ())
            .context("deleting event via CalDAV")?;
        match reply.status {
            200..=299 | 404 => Ok(()),
            412 if !reply.is_caldav_error() => Err(StoreError::Conflict),
            _ => Err(reply.error("deleting event via CalDAV").into()),
        }
    }
}

/// The resources of a WebDAV multistatus response as (href, ETag, calendar data).
fn parse_multistatus(xml: &str) -> Result<Vec<(String, Option<String>, String)>> {
    let document = roxmltree::Document::parse(xml).context("parsing the server's query result")?;
    let text_of = |response: roxmltree::Node, name: &str| {
        response
            .descendants()
            .find(|node| node.tag_name().name() == name)
            .and_then(|node| node.text())
            .map(|text| text.trim().to_string())
    };
    Ok(document
        .descendants()
        .filter(|node| node.tag_name().name() == "response")
        .filter_map(|response| {
            Some((
                text_of(response, "href")?,
                text_of(response, "getetag"),
                text_of(response, "calendar-data")?,
            ))
        })
        .collect())
}

fn xml_escape(text: &str) -> String {
    text.replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
        .replace('"', "&quot;")
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

/// `https://host:port` of a URL, without any path.
fn origin(server_url: &str) -> String {
    match server_url.split_once("://") {
        Some((scheme, rest)) => format!("{scheme}://{}", rest.split('/').next().unwrap_or(rest)),
        None => server_url.trim_end_matches('/').to_string(),
    }
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
        assert_eq!(
            origin("https://dav.example.com:8443/caldav/"),
            "https://dav.example.com:8443"
        );

        let storage = CalDavStorage::new("http://localhost:5232", "test", "pass", "/cal/");
        assert_eq!(
            storage.event_url("abc:1+2@example.com"),
            "http://localhost:5232/cal/abc:1+2@example.com.ics"
        );
        // "test:pass" in base64
        assert_eq!(storage.authorization, "Basic dGVzdDpwYXNz");
    }

    #[test]
    fn resource_names_are_percent_encoded() {
        let storage = CalDavStorage::new("http://localhost:5232", "test", "pass", "/cal/");
        assert_eq!(
            storage.event_url("Weekly Meeting-host/a?b#c-0"),
            "http://localhost:5232/cal/Weekly%20Meeting-host%2Fa%3Fb%23c-0.ics"
        );
        assert_eq!(
            storage.event_url("Besprechung-ä"),
            "http://localhost:5232/cal/Besprechung-%C3%A4.ics"
        );
    }

    #[test]
    fn parses_query_results() {
        let xml = r#"<?xml version="1.0"?>
<d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav">
  <d:response>
    <d:href>/cal/some-other-name.ics</d:href>
    <d:propstat>
      <d:prop>
        <d:getetag>"abc"</d:getetag>
        <cal:calendar-data>BEGIN:VCALENDAR
END:VCALENDAR</cal:calendar-data>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
  <d:response><d:href>/cal/</d:href></d:response>
</d:multistatus>"#;
        let resources = parse_multistatus(xml).unwrap();
        assert_eq!(resources.len(), 1);
        assert_eq!(resources[0].0, "/cal/some-other-name.ics");
        assert_eq!(resources[0].1.as_deref(), Some("\"abc\""));
        assert!(resources[0].2.starts_with("BEGIN:VCALENDAR"));
        assert!(parse_multistatus("not xml").is_err());
        assert_eq!(xml_escape("a&b<c>\"d\""), "a&amp;b&lt;c&gt;&quot;d&quot;");
    }

    #[test]
    fn errors_carry_the_servers_reason() {
        let reply = Reply {
            status: 400,
            etag: None,
            body: "<error>\n  CAL-4034 Invalid calendar user\n</error>".to_string(),
        };
        assert_eq!(
            reply.error("storing event via CalDAV").to_string(),
            "storing event via CalDAV: HTTP status 400: <error> CAL-4034 Invalid calendar user </error>"
        );
        let empty = Reply {
            status: 403,
            etag: None,
            body: String::new(),
        };
        assert!(!empty.is_caldav_error());
        assert!(
            !reply.is_caldav_error(),
            "an error without a CalDAV precondition"
        );
        // What Xandikos sends with status 412 for invalid calendar data.
        let invalid = Reply {
            status: 412,
            etag: None,
            body: r#"<ns0:error xmlns:ns0="DAV:" xmlns:ns1="urn:ietf:params:xml:ns:caldav"><ns1:valid-calendar-data /><ns0:responsedescription>Not a valid calendar file</ns0:responsedescription></ns0:error>"#.to_string(),
        };
        assert!(invalid.is_caldav_error());
        assert_eq!(empty.error("x").to_string(), "x: HTTP status 403");
    }
}
