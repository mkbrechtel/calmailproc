//! Validation of calendars and UIDs.
//!
//! The cardinality rules are the ones go-ical enforced when encoding, which the
//! Go version of calmailproc relied on to reject broken invitations.

use super::{Calendar, Component, decode};
use crate::error::{Context, Error, Result};

/// Checks how often properties occur and which components may be nested.
pub fn validate(calendar: &Calendar) -> Result<()> {
    check_component(calendar)?;
    calendar.components.iter().try_for_each(validate)
}

/// Checks that calendar text can be parsed and written back, i.e. stored.
pub fn validate_event(ics: &str) -> Result<()> {
    let calendar = decode(ics).context("decoding calendar")?;
    validate(&calendar).context("encoding calendar")
}

/// Checks a UID: not empty, at most 255 bytes, no control characters.
///
/// RFC 5545 defines UID as a TEXT value, so anything else is allowed,
/// including spaces (OpenProject) and slashes. Storage is responsible for
/// turning a UID into a safe resource name.
pub fn validate_uid(uid: &str) -> Result<()> {
    if uid.is_empty() {
        return Err(Error::new("UID cannot be empty"));
    }
    if uid.len() > 255 {
        return Err(Error::new(format!(
            "UID exceeds maximum length of 255 characters (length: {})",
            uid.len()
        )));
    }
    if uid.contains(['\r', '\n']) {
        return Err(Error::new("UID contains line breaks which are not allowed"));
    }
    if uid.chars().any(char::is_control) {
        return Err(Error::new(
            "UID contains control characters which are not allowed",
        ));
    }
    Ok(())
}

fn check_component(component: &Component) -> Result<()> {
    let name = component.name.as_str();
    let fail = |reason: String| Err(Error::new(format!("failed to encode {name}: {reason}")));
    let count = |property: &str| component.props(property).count();
    let only_alarms = || {
        component
            .components
            .iter()
            .find(|child| child.name != "VALARM")
            .map(|child| {
                format!(
                    "nested \"{}\" components are forbidden, only VALARM is allowed",
                    child.name
                )
            })
    };

    let (exactly_one, at_most_one): (&[&str], &[&str]) = match name {
        "VCALENDAR" => {
            if component.components.is_empty() {
                return fail("calendar is empty".to_string());
            }
            (
                &["PRODID", "VERSION"],
                &[
                    "CALSCALE",
                    "METHOD",
                    "UID",
                    "LAST-MODIFIED",
                    "URL",
                    "REFRESH-INTERVAL",
                    "SOURCE",
                    "COLOR",
                ],
            )
        }
        "VEVENT" => {
            if let Some(reason) = only_alarms() {
                return fail(reason);
            }
            if count("DTEND") > 0 && count("DURATION") > 0 {
                return fail("only one of DTEND and DURATION can be specified".to_string());
            }
            (
                &["DTSTAMP", "UID"],
                &[
                    "DTSTART",
                    "CLASS",
                    "CREATED",
                    "DESCRIPTION",
                    "GEO",
                    "LAST-MODIFIED",
                    "LOCATION",
                    "ORGANIZER",
                    "PRIORITY",
                    "RRULE",
                    "SEQUENCE",
                    "STATUS",
                    "SUMMARY",
                    "TRANSP",
                    "URL",
                    "RECURRENCE-ID",
                    "DTEND",
                    "DURATION",
                    "COLOR",
                ],
            )
        }
        "VTODO" => {
            if let Some(reason) = only_alarms() {
                return fail(reason);
            }
            if count("DUE") > 0 && count("DURATION") > 0 {
                return fail("only one of DUE and DURATION can be specified".to_string());
            }
            if count("DURATION") > 0 && count("DTSTART") == 0 {
                return fail("DTSTART is required when DURATION is specified".to_string());
            }
            (
                &["DTSTAMP", "UID"],
                &[
                    "CLASS",
                    "COMPLETED",
                    "CREATED",
                    "DESCRIPTION",
                    "DTSTART",
                    "GEO",
                    "LAST-MODIFIED",
                    "LOCATION",
                    "ORGANIZER",
                    "PERCENT-COMPLETE",
                    "PRIORITY",
                    "RECURRENCE-ID",
                    "SEQUENCE",
                    "STATUS",
                    "SUMMARY",
                    "URL",
                    "DUE",
                    "DURATION",
                    "COLOR",
                ],
            )
        }
        "VJOURNAL" => {
            if !component.components.is_empty() {
                return fail("nested components are forbidden".to_string());
            }
            (
                &["DTSTAMP", "UID"],
                &[
                    "CLASS",
                    "CREATED",
                    "DTSTART",
                    "LAST-MODIFIED",
                    "ORGANIZER",
                    "RECURRENCE-ID",
                    "SEQUENCE",
                    "STATUS",
                    "SUMMARY",
                    "URL",
                    "COLOR",
                ],
            )
        }
        "VFREEBUSY" => {
            if !component.components.is_empty() {
                return fail("nested components are forbidden".to_string());
            }
            (
                &["DTSTAMP", "UID"],
                &["CONTACT", "DTSTART", "DTEND", "ORGANIZER", "URL"],
            )
        }
        "VTIMEZONE" => {
            if component.components.is_empty() {
                return fail("expected one nested STANDARD or DAYLIGHT component".to_string());
            }
            if let Some(child) = component
                .components
                .iter()
                .find(|c| c.name != "STANDARD" && c.name != "DAYLIGHT")
            {
                return fail(format!(
                    "nested \"{}\" components are forbidden, only STANDARD and DAYLIGHT are allowed",
                    child.name
                ));
            }
            (&["TZID"], &["LAST-MODIFIED", "TZURL"])
        }
        "STANDARD" | "DAYLIGHT" => (&["DTSTART", "TZOFFSETTO", "TZOFFSETFROM"], &[]),
        _ => (&[], &[]),
    };

    for property in exactly_one {
        let n = count(property);
        if n != 1 {
            return fail(format!("want exactly one {property} property, got {n}"));
        }
    }
    for property in at_most_one {
        let n = count(property);
        if n > 1 {
            return fail(format!("want at most one {property} property, got {n}"));
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn calendar(event_lines: &str) -> String {
        format!(
            "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\nMETHOD:REQUEST\nBEGIN:VEVENT\n{event_lines}END:VEVENT\nEND:VCALENDAR\n"
        )
    }

    #[test]
    fn valid_event_passes() {
        let ics = calendar(
            "UID:test-uid-123\nSUMMARY:Valid Test Event\nDTSTAMP:20250415T100000Z\nDTSTART:20250415T100000Z\nDTEND:20250415T110000Z\n",
        );
        validate_event(&ics).unwrap();
    }

    #[test]
    fn missing_dtstamp_fails() {
        let ics = calendar("UID:test-uid-456\nDTSTART:20250415T100000Z\nDTEND:20250415T110000Z\n");
        let error = validate_event(&ics).unwrap_err().to_string();
        assert!(
            error.contains("want exactly one DTSTAMP property, got 0"),
            "{error}"
        );
    }

    #[test]
    fn dtend_and_duration_together_fail() {
        let ics = calendar(
            "UID:test-uid-789\nDTSTAMP:20250415T100000Z\nDTSTART:20250415T100000Z\nDTEND:20250415T110000Z\nDURATION:PT1H\n",
        );
        let error = validate_event(&ics).unwrap_err().to_string();
        assert!(error.contains("DTEND and DURATION"), "{error}");
    }

    #[test]
    fn duplicate_url_and_missing_prodid_fail() {
        let two_urls = calendar("UID:u\nDTSTAMP:20250415T100000Z\nURL:http://a\nURL:http://b\n");
        let error = validate_event(&two_urls).unwrap_err().to_string();
        assert!(
            error.contains("want at most one URL property, got 2"),
            "{error}"
        );

        let no_prodid = calendar("UID:u\nDTSTAMP:20250415T100000Z\n")
            .replace("PRODID:-//hacksw/handcal//NONSGML v1.0//EN\n", "");
        let error = validate_event(&no_prodid).unwrap_err().to_string();
        assert!(
            error.contains("want exactly one PRODID property, got 0"),
            "{error}"
        );
    }

    #[test]
    fn nested_components_are_checked() {
        let bad_timezone = "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:x\nBEGIN:VTIMEZONE\nTZID:Europe/Berlin\nEND:VTIMEZONE\nEND:VCALENDAR\n";
        assert!(validate_event(bad_timezone).is_err());
        let empty = "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:x\nEND:VCALENDAR\n";
        assert!(
            validate_event(empty)
                .unwrap_err()
                .to_string()
                .contains("calendar is empty")
        );
    }

    #[test]
    fn uid_validation() {
        let valid = [
            "abc123",
            "user@example.com",
            "event-123_test",
            "event.123:test",
            "event+123",
            // UID is a TEXT value: these are unusual but allowed.
            "Weekly Meeting-host.example-name-0",
            "event/123",
            "event#123!",
            "Besprechung-äöü",
            &"a".repeat(255),
        ];
        for uid in valid {
            assert!(validate_uid(uid).is_ok(), "{uid} should be valid");
        }

        let invalid = [
            ("", "UID cannot be empty"),
            (&"a".repeat(256) as &str, "exceeds maximum length"),
            ("event\n123", "line breaks"),
            ("event\r123", "line breaks"),
            ("event\x00123", "control characters"),
            ("event\t123", "control characters"),
        ];
        for (uid, message) in invalid {
            let error = validate_uid(uid).unwrap_err().to_string();
            assert!(error.contains(message), "{uid:?}: {error}");
        }
    }
}
