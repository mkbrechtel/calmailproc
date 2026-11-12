//! Email parsing: header fields and the calendar attachment, if any.
//!
//! MIME structure, transfer encodings and charsets are handled by
//! `mail-parser`; the calendar text is handed to the [`ical`](crate::ical)
//! module.

use mail_parser::{AddressList, MessageParser};

use crate::error::{Error, Result};
use crate::ical::{self, Event};

/// A parsed email with its calendar event, if it has one.
#[derive(Debug, Clone, Default)]
pub struct Email {
    pub subject: String,
    pub from: String,
    pub to: String,
    /// The Date header as RFC 3339, if present and valid.
    pub date: Option<String>,
    pub event: Option<Event>,
}

impl Email {
    pub fn has_calendar(&self) -> bool {
        self.event.is_some()
    }
}

/// Parses a raw RFC 822 message. A calendar part that cannot be parsed is an
/// error; a mail without any calendar part is not.
pub fn parse(raw: &[u8]) -> Result<Email> {
    let message = MessageParser::default()
        .parse(raw)
        .ok_or_else(|| Error::new("reading message: not an email"))?;

    let mut email = Email {
        subject: message.subject().unwrap_or_default().to_string(),
        from: message.from().map(format_addresses).unwrap_or_default(),
        to: message.to().map(format_addresses).unwrap_or_default(),
        date: message.date().map(|date| date.to_rfc3339()),
        event: None,
    };

    // Parts of attached messages (message/rfc822) are not searched: a forwarded
    // mail's invitation is only used if it is part of the outer message.
    let mut first_error = None;
    for part in message.root().parts() {
        // A multipart is a container, whatever its Content-Type says.
        // OpenProject puts `method=` and `name=....ics` on the outer
        // multipart/mixed (test 20).
        if part.is_multipart() {
            continue;
        }
        let is_calendar = part.is_content_type("text", "calendar")
            || part.is_content_type("application", "ics")
            || part.is_content_type("text", "x-vcalendar")
            || part
                .attachment_name()
                .is_some_and(|name| name.to_ascii_lowercase().ends_with(".ics"));
        if !is_calendar {
            continue;
        }
        let text = match part.text() {
            Some(text) => text.into_owned(),
            None => String::from_utf8_lossy(&part.decoded()).into_owned(),
        };
        // Mails often carry the same calendar twice, as text/calendar and as
        // an attachment. The last copy that can be parsed wins; a broken copy
        // only matters if no copy can be parsed (tests 21 and 22).
        match ical::parse_ical_data(&text) {
            Ok(event) => email.event = Some(event),
            Err(error) => {
                let content_type = part
                    .content_type()
                    .map(|c| format!("{}/{}", c.ctype(), c.subtype().unwrap_or_default()))
                    .unwrap_or_else(|| "unknown".to_string());
                first_error.get_or_insert(Error::new(format!(
                    "extracting calendar data: in {content_type} part: {error}"
                )));
            }
        }
    }
    if let (None, Some(error)) = (&email.event, first_error) {
        return Err(error);
    }

    Ok(email)
}

fn format_addresses(list: AddressList) -> String {
    list.mailboxes()
        .map(|mailbox| match (mailbox.name(), mailbox.address()) {
            (Some(name), Some(address)) => format!("{name} <{address}>"),
            (None, Some(address)) => address.to_string(),
            (Some(name), None) => name.to_string(),
            (None, None) => String::new(),
        })
        .collect::<Vec<_>>()
        .join(", ")
}

#[cfg(test)]
mod tests {
    use super::*;

    fn read(name: &str) -> Vec<u8> {
        std::fs::read(format!("test/maildir/cur/{name}")).unwrap()
    }

    #[test]
    fn parses_headers_and_calendar() {
        let email = parse(&read("test-01-1.eml")).unwrap();
        assert!(!email.subject.is_empty());
        assert!(!email.from.is_empty());
        assert!(!email.to.is_empty());
        assert!(email.date.is_some());
        assert!(email.has_calendar());
        let event = email.event.unwrap();
        assert!(!event.uid.is_empty());
        assert_eq!(event.summary, "Test Event 1");
        assert_eq!(event.method, "REQUEST");
    }

    #[test]
    fn mail_without_calendar_is_not_an_error() {
        let email = parse(&read("test-0.eml")).unwrap();
        assert!(!email.has_calendar());
    }

    #[test]
    fn broken_calendar_is_an_error() {
        let error = parse(&read("test-12.eml")).unwrap_err().to_string();
        assert!(error.contains("extracting calendar data"), "{error}");
    }

    fn calendar(uid: &str) -> String {
        format!(
            "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\nUID:{uid}\r\nDTSTAMP:20250101T000000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
        )
    }

    #[test]
    fn calendar_parameters_on_a_multipart_do_not_make_it_a_calendar() {
        // OpenProject (test 20): the container itself is named like an .ics file.
        let mail = format!(
            "From: a@example.com\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=\"b\"; charset=UTF-8; method=PUBLISH; name=meeting.ics\r\n\r\n--b\r\nContent-Type: text/plain\r\n\r\nHello\r\n--b\r\nContent-Type: text/calendar; charset=UTF-8\r\n\r\n{}--b--\r\n",
            calendar("in-part")
        );
        let event = parse(mail.as_bytes()).unwrap().event.unwrap();
        assert_eq!(event.uid, "in-part");
    }

    #[test]
    fn a_broken_calendar_copy_is_skipped_if_another_copy_is_valid() {
        let broken = calendar("copy").replace("UID:copy", "UID;broken");
        for (first, second) in [(&broken, &calendar("copy")), (&calendar("copy"), &broken)] {
            let mail = format!(
                "From: a@example.com\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=\"b\"\r\n\r\n--b\r\nContent-Type: text/calendar\r\n\r\n{first}--b\r\nContent-Type: application/ics; name=\"invite.ics\"\r\n\r\n{second}--b--\r\n"
            );
            let event = parse(mail.as_bytes()).unwrap().event.unwrap();
            assert_eq!(event.uid, "copy");
        }

        // If every copy is broken, the error names the part.
        let mail = format!(
            "From: a@example.com\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=\"b\"\r\n\r\n--b\r\nContent-Type: text/calendar\r\n\r\n{broken}--b--\r\n"
        );
        let error = parse(mail.as_bytes()).unwrap_err().to_string();
        assert!(error.contains("in text/calendar part"), "{error}");
    }

    #[test]
    fn decodes_base64_and_quoted_printable_calendar_parts() {
        let ics = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nMETHOD:REQUEST\r\nBEGIN:VEVENT\r\nUID:encoded-event\r\nSUMMARY:Caf=C3=A9 meeting\r\nDTSTAMP:20250101T000000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n";
        let quoted_printable = format!(
            "From: a@example.com\r\nTo: b@example.com\r\nSubject: qp\r\nMIME-Version: 1.0\r\nContent-Type: text/calendar; charset=UTF-8; method=REQUEST\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n{ics}"
        );
        let event = parse(quoted_printable.as_bytes()).unwrap().event.unwrap();
        assert_eq!(event.uid, "encoded-event");
        assert_eq!(event.summary, "Café meeting");

        // test-05-2 carries its calendar base64 encoded.
        let event = parse(&read("test-05-2.eml")).unwrap().event.unwrap();
        assert_eq!(event.method, "REPLY");
    }

    #[test]
    fn finds_calendar_by_attachment_name() {
        let mail = "From: a@example.com\r\nSubject: attachment\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=\"b\"\r\n\r\n--b\r\nContent-Type: text/plain\r\n\r\nSee attachment.\r\n--b\r\nContent-Type: application/octet-stream; name=\"invite.ics\"\r\nContent-Disposition: attachment; filename=\"invite.ics\"\r\n\r\nBEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\nUID:by-name\r\nDTSTAMP:20250101T000000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n--b--\r\n";
        let event = parse(mail.as_bytes()).unwrap().event.unwrap();
        assert_eq!(event.uid, "by-name");
    }
}
