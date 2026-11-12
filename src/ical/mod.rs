//! iCalendar parsing and writing.
//!
//! This module is the only place that touches the `icalendar` crate. It exposes
//! a small owned tree ([`Component`] / [`Property`]) that the processor edits,
//! plus [`decode`] and [`encode`].
//!
//! Three things the `icalendar` crate does not do for us, found in the
//! feasibility spike and handled here:
//!
//! 1. Its parser accepts a content line without `:` and merges the next line
//!    into it. [`decode`] rejects such input before parsing.
//! 2. Its parser tree cannot be written back as is: TEXT values are unescaped
//!    and parameter quotes stripped on parsing. [`encode`] writes every
//!    property through `icalendar::Property`, which restores both.
//! 3. It does not check how often a property may occur. [`validate`] carries
//!    the rules the Go version got from go-ical's encoder.
//!
//! Written output is semantically equal to the input but not byte-identical:
//! parameters are sorted and quoting/escaping is normalised.

mod compare;
mod validate;

pub use compare::{Comparison, compare_components, compare_events, parse_ical_time, sequence};
pub use validate::{validate, validate_event, validate_uid};

use std::panic::{AssertUnwindSafe, catch_unwind};

use icalendar::parser as ic;

use crate::error::{Context, Error, Result};

/// One content line: `NAME;PARAM=value:VALUE`.
///
/// `value` is unescaped for TEXT properties (`SUMMARY`, `DESCRIPTION`, ...).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Property {
    pub name: String,
    pub params: Vec<(String, String)>,
    pub value: String,
}

impl Property {
    pub fn new(name: &str, value: &str) -> Self {
        Property {
            name: name.to_string(),
            params: Vec::new(),
            value: value.to_string(),
        }
    }

    pub fn param(&self, key: &str) -> Option<&str> {
        self.params
            .iter()
            .find(|(k, _)| k.eq_ignore_ascii_case(key))
            .map(|(_, v)| v.as_str())
    }

    pub fn remove_param(&mut self, key: &str) {
        self.params.retain(|(k, _)| !k.eq_ignore_ascii_case(key));
    }

    pub fn set_param(&mut self, key: &str, value: &str) {
        match self
            .params
            .iter_mut()
            .find(|(k, _)| k.eq_ignore_ascii_case(key))
        {
            Some((_, v)) => *v = value.to_string(),
            None => self.params.push((key.to_string(), value.to_string())),
        }
    }
}

/// A component such as `VCALENDAR`, `VEVENT` or `VTIMEZONE`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Component {
    pub name: String,
    pub properties: Vec<Property>,
    pub components: Vec<Component>,
}

/// The root `VCALENDAR` component.
pub type Calendar = Component;

impl Component {
    pub fn new(name: &str) -> Self {
        Component {
            name: name.to_string(),
            properties: Vec::new(),
            components: Vec::new(),
        }
    }

    /// The first property with this name.
    pub fn prop(&self, name: &str) -> Option<&Property> {
        self.properties.iter().find(|p| p.name == name)
    }

    /// The value of the first property with this name.
    pub fn value(&self, name: &str) -> Option<&str> {
        self.prop(name).map(|p| p.value.as_str())
    }

    /// All properties with this name.
    pub fn props<'a>(&'a self, name: &'a str) -> impl Iterator<Item = &'a Property> {
        self.properties.iter().filter(move |p| p.name == name)
    }

    pub fn has_prop(&self, name: &str) -> bool {
        self.prop(name).is_some()
    }

    /// Replaces every property of this name with a single one.
    pub fn set_prop(&mut self, name: &str, value: &str) {
        let position = self.properties.iter().position(|p| p.name == name);
        self.properties.retain(|p| p.name != name);
        let property = Property::new(name, value);
        match position {
            Some(index) => self.properties.insert(index, property),
            None => self.properties.push(property),
        }
    }

    pub fn remove_props(&mut self, name: &str) {
        self.properties.retain(|p| p.name != name);
    }

    /// The `VEVENT` children.
    pub fn events(&self) -> impl Iterator<Item = &Component> {
        self.components.iter().filter(|c| c.name == "VEVENT")
    }

    pub fn events_mut(&mut self) -> impl Iterator<Item = &mut Component> {
        self.components.iter_mut().filter(|c| c.name == "VEVENT")
    }
}

/// What the processor needs to know about a calendar mail.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Event {
    pub uid: String,
    /// The complete iCalendar text.
    pub raw_data: String,
    pub summary: String,
    /// Calendar method (`REQUEST`, `REPLY`, `CANCEL`, ...), empty if absent.
    pub method: String,
    pub sequence: i64,
}

impl Event {
    /// True if any `VEVENT` has a `RECURRENCE-ID`, i.e. the data contains an
    /// update to a single occurrence of a recurring event.
    pub fn is_recurring_update(&self) -> bool {
        decode(&self.raw_data)
            .map(|cal| cal.events().any(|e| e.has_prop("RECURRENCE-ID")))
            .unwrap_or(false)
    }
}

/// Parses iCalendar text into a calendar tree. Does not check cardinality;
/// see [`validate`].
///
/// Empty lines are ignored. The standard does not allow them, but Horde puts
/// them in the middle of folded lines (tests 21 and 22); without the empty
/// line the continuation joins the line before it again.
pub fn decode(ics: &str) -> Result<Calendar> {
    // Line endings are normalised too: mails mix CRLF and LF.
    let lines: Vec<&str> = ics.lines().filter(|line| !line.is_empty()).collect();
    let unfolded = ic::unfold(&lines.join("\n"));
    // The parser is lenient in ways that corrupt data: it merges a line
    // without ':' into the next one and splits a name at a space. Check the
    // basic shape of every content line first.
    for (number, line) in unfolded.lines().enumerate() {
        let name = line.split([';', ':']).next().unwrap_or_default();
        let problem = if !line.contains(':') {
            "without ':'"
        } else if name.is_empty() || !name.chars().all(|c| c.is_ascii_alphanumeric() || c == '-') {
            "with an invalid property name"
        } else {
            continue;
        };
        return Err(Error::new(format!(
            "decoding iCal data: malformed content line {} {problem}: {}",
            number + 1,
            truncate(line.trim(), 60)
        )));
    }

    // The Go version had to recover from panics in its iCalendar library on
    // malformed mails. None were seen with this parser, but a broken
    // attachment must never take down a whole maildir run.
    let roots = catch_unwind(AssertUnwindSafe(|| ic::read_components(&unfolded)))
        .map_err(|_| Error::new("decoding iCal data: panic in decoder"))?
        .map_err(|e| Error::new(format!("decoding iCal data: {}", describe(&e, &unfolded))))?;

    let root = roots
        .into_iter()
        .find(|c| c.name == "VCALENDAR")
        .ok_or_else(|| Error::new("decoding iCal data: no VCALENDAR component found"))?;
    let calendar = to_owned(root);
    check_no_line_breaks(&calendar)?;
    Ok(calendar)
}

/// The parser reports only "at line N, in Satisfy". Add the text of that line
/// (of the unfolded data), which is what one needs to find the problem.
fn describe(parser_error: &str, unfolded: &str) -> String {
    let message = first_line(parser_error);
    let line = message
        .split("at line ")
        .nth(1)
        .and_then(|rest| rest.split(|c: char| !c.is_ascii_digit()).next())
        .and_then(|number| number.parse::<usize>().ok())
        .and_then(|number| unfolded.lines().nth(number.saturating_sub(1)));
    match line {
        Some(line) => format!("{message} {}", truncate(line.trim(), 60)),
        None => message,
    }
}

/// Validates a calendar and writes it as iCalendar text.
pub fn encode(calendar: &Calendar) -> Result<String> {
    validate(calendar).context("encoding calendar")?;
    let mut out = String::new();
    write_component(&mut out, calendar);
    Ok(out)
}

/// Parses iCalendar text and extracts the fields of its first `VEVENT`.
pub fn parse_ical_data(ics: &str) -> Result<Event> {
    let calendar = decode(ics).context("parsing iCal data")?;
    let vevent = calendar
        .events()
        .next()
        .ok_or_else(|| Error::new("no VEVENT component found in iCalendar data"))?;

    let uid = vevent
        .value("UID")
        .ok_or_else(|| Error::new("VEVENT missing required UID property"))?;
    validate_uid(uid).context("invalid UID")?;

    Ok(Event {
        uid: uid.to_string(),
        raw_data: ics.to_string(),
        summary: vevent
            .value("SUMMARY")
            .unwrap_or("Event without summary")
            .to_string(),
        method: calendar.value("METHOD").unwrap_or_default().to_string(),
        sequence: vevent
            .value("SEQUENCE")
            .and_then(|s| s.trim().parse().ok())
            .unwrap_or(0),
    })
}

fn to_owned(component: ic::Component) -> Component {
    Component {
        name: component.name.to_string(),
        properties: component
            .properties
            .into_iter()
            .map(|p| Property {
                name: p.name.to_string(),
                params: p
                    .params
                    .into_iter()
                    .map(|param| {
                        (
                            param.key.to_string(),
                            param.val.map(|v| v.to_string()).unwrap_or_default(),
                        )
                    })
                    .collect(),
                value: p.val.to_string(),
            })
            .collect(),
        components: component.components.into_iter().map(to_owned).collect(),
    }
}

/// A line break in a name or parameter means the parser ran two content lines
/// together. Reject it; writing it back would produce a corrupt calendar.
fn check_no_line_breaks(component: &Component) -> Result<()> {
    let broken = |s: &str| s.contains(['\r', '\n']);
    for property in &component.properties {
        if broken(&property.name) || property.params.iter().any(|(k, v)| broken(k) || broken(v)) {
            return Err(Error::new(format!(
                "decoding iCal data: malformed {} property in {}",
                first_line(&property.name),
                component.name
            )));
        }
    }
    component
        .components
        .iter()
        .try_for_each(check_no_line_breaks)
}

fn write_component(out: &mut String, component: &Component) {
    out.push_str(&format!("BEGIN:{}\r\n", component.name));
    for property in &component.properties {
        let mut line = icalendar::Property::new(&property.name, &property.value);
        for (key, value) in &property.params {
            line.add_parameter(key, value);
        }
        // Escapes TEXT values, quotes parameter values and folds long lines.
        let text: String = line.try_into().expect("writing to a String cannot fail");
        out.push_str(&text);
    }
    for child in &component.components {
        write_component(out, child);
    }
    out.push_str(&format!("END:{}\r\n", component.name));
}

fn first_line(text: &str) -> String {
    truncate(
        text.lines().find(|l| !l.trim().is_empty()).unwrap_or(""),
        120,
    )
}

fn truncate(text: &str, max: usize) -> String {
    if text.chars().count() <= max {
        text.to_string()
    } else {
        format!("{}...", text.chars().take(max).collect::<String>())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    const SIMPLE: &str = "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\nMETHOD:REQUEST\nBEGIN:VEVENT\nUID:test-uid-123\nSUMMARY:Test Event\nSEQUENCE:0\nEND:VEVENT\nEND:VCALENDAR\n";

    #[test]
    fn parse_ical_data_extracts_fields() {
        let event = parse_ical_data(SIMPLE).unwrap();
        assert_eq!(event.uid, "test-uid-123");
        assert_eq!(event.summary, "Test Event");
        assert_eq!(event.method, "REQUEST");
        assert_eq!(event.sequence, 0);
        assert_eq!(event.raw_data, SIMPLE);
        assert!(!event.is_recurring_update());
    }

    #[test]
    fn parse_ical_data_defaults() {
        let ics = SIMPLE
            .replace("SUMMARY:Test Event\n", "")
            .replace("METHOD:REQUEST\n", "")
            .replace("SEQUENCE:0", "SEQUENCE:7");
        let event = parse_ical_data(&ics).unwrap();
        assert_eq!(event.summary, "Event without summary");
        assert_eq!(event.method, "");
        assert_eq!(event.sequence, 7);
    }

    #[test]
    fn parse_ical_data_requires_vevent_and_uid() {
        let no_uid = SIMPLE.replace("UID:test-uid-123\n", "");
        assert!(
            parse_ical_data(&no_uid)
                .unwrap_err()
                .to_string()
                .contains("UID")
        );
        let no_event = "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:x\nEND:VCALENDAR\n";
        assert!(
            parse_ical_data(no_event)
                .unwrap_err()
                .to_string()
                .contains("VEVENT")
        );
        assert!(parse_ical_data("this is not a calendar").is_err());
    }

    #[test]
    fn decode_rejects_line_without_colon() {
        // From test-12: the parser would merge the UID line into the CN parameter.
        let ics = SIMPLE.replace("SUMMARY:Test Event", "ORGANIZER;CN=Sixt SE");
        let error = decode(&ics).unwrap_err().to_string();
        assert!(
            error.contains("malformed content line 7 without ':': ORGANIZER;CN=Sixt SE"),
            "{error}"
        );
    }

    #[test]
    fn decode_ignores_empty_lines_even_inside_folded_lines() {
        // Horde (tests 21, 22): an empty line in the middle of a folded line.
        let ics = "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:x\n\nBEGIN:VEVENT\nUID:folded\nATTENDEE;ROLE=REQ-PARTICIPANT;CN=someone@exam\n\n ple.com:mailto:someone@example.com\r\nDESCRIPTION:first\r\n\r\n  second\nEND:VEVENT\nEND:VCALENDAR\n";
        let calendar = decode(ics).unwrap();
        let event = calendar.events().next().unwrap();
        let attendee = event.prop("ATTENDEE").unwrap();
        assert_eq!(attendee.param("CN"), Some("someone@example.com"));
        assert_eq!(attendee.value, "mailto:someone@example.com");
        assert_eq!(event.value("DESCRIPTION"), Some("first second"));
    }

    #[test]
    fn decode_rejects_invalid_property_names() {
        // The parser would read this as an empty "SUM" and a "MARY" property.
        let ics = SIMPLE.replace("SUMMARY:Test Event", "SUM MARY:broken name");
        let error = decode(&ics).unwrap_err().to_string();
        assert!(
            error.contains("line 7 with an invalid property name: SUM MARY:broken name"),
            "{error}"
        );
    }

    #[test]
    fn parser_errors_show_the_offending_line() {
        let error = describe(
            "error: 0: at line 3, in Satisfy:",
            "A:1\nB:2\nC:the culprit\nD:4",
        );
        assert_eq!(error, "error: 0: at line 3, in Satisfy: C:the culprit");
        assert_eq!(describe("something else", "A:1"), "something else");
    }

    #[test]
    fn recurring_update_is_detected() {
        let ics = SIMPLE.replace("SEQUENCE:0", "RECURRENCE-ID:20250101T100000Z");
        assert!(parse_ical_data(&ics).unwrap().is_recurring_update());
    }

    #[test]
    fn encode_round_trips_escaping_quoting_and_folding() {
        let long = "x".repeat(200);
        let ics = format!(
            "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VTIMEZONE\r\nTZID:(UTC+01:00) Amsterdam\\, Berlin\r\nBEGIN:STANDARD\r\nDTSTART:16010101T030000\r\nTZOFFSETFROM:+0200\r\nTZOFFSETTO:+0100\r\nEND:STANDARD\r\nEND:VTIMEZONE\r\nBEGIN:VEVENT\r\nUID:round-trip\r\nDTSTAMP:20250101T000000Z\r\nSUMMARY:Lunch\\, then work\\; maybe\r\nDESCRIPTION:line one\\nline two {long}\r\nATTENDEE;CN=\"Doe, John\";PARTSTAT=NEEDS-ACTION:mailto:john@example.com\r\nX-CUSTOM;X-PARAM=1:kept as is\\, really\r\nBEGIN:VALARM\r\nTRIGGER:-PT15M\r\nACTION:DISPLAY\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
        );
        let calendar = decode(&ics).unwrap();
        let event = calendar.events().next().unwrap();
        assert_eq!(event.value("SUMMARY"), Some("Lunch, then work; maybe"));
        assert_eq!(
            event.prop("ATTENDEE").unwrap().param("CN"),
            Some("Doe, John")
        );
        assert_eq!(event.components[0].name, "VALARM");

        let written = encode(&calendar).unwrap();
        assert!(written.contains("SUMMARY:Lunch\\, then work\\; maybe\r\n"));
        assert!(written.contains("CN=\"Doe, John\""));
        assert!(
            written.lines().all(|line| line.len() <= 75),
            "lines are folded"
        );
        assert_eq!(decode(&written).unwrap(), sorted_params(calendar));
    }

    #[test]
    fn set_prop_replaces_in_place_and_remove_props_removes() {
        let mut calendar = decode(SIMPLE).unwrap();
        calendar.remove_props("METHOD");
        assert!(!calendar.has_prop("METHOD"));
        let event = calendar.events_mut().next().unwrap();
        event.set_prop("SUMMARY", "Changed");
        event.set_prop("STATUS", "CANCELLED");
        let names: Vec<&str> = event.properties.iter().map(|p| p.name.as_str()).collect();
        assert_eq!(names, ["UID", "SUMMARY", "SEQUENCE", "STATUS"]);
        assert_eq!(event.value("SUMMARY"), Some("Changed"));
    }

    /// The writer sorts parameters, so compare against a sorted copy.
    fn sorted_params(mut component: Component) -> Component {
        for property in &mut component.properties {
            property.params.sort();
        }
        component.components = component
            .components
            .into_iter()
            .map(sorted_params)
            .collect();
        component
    }
}
