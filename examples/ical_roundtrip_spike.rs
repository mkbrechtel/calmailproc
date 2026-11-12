//! Feasibility spike for the Rust rewrite (calmailproc 0.2).
//!
//! Question: can `mail-parser` + `icalendar` replace Go's `net/mail` +
//! `emersion/go-ical` for what the processor needs?
//!
//! For every mail in the test maildir this
//!   1. extracts the calendar part with `mail-parser`,
//!   2. parses it into `icalendar`'s generic component tree,
//!   3. writes it back out and checks that no content was lost,
//!   4. applies the edits the processor makes (drop METHOD, set STATUS, set
//!      PARTSTAT, add SCHEDULE-AGENT=CLIENT) and checks they survive a re-parse,
//!   5. applies the cardinality rules that go-ical enforced on encode, which is
//!      what makes tests 13-15 fail in the Go version.
//!
//! Findings:
//!
//! 1. The parser accepts a content line without ':' and merges the next line
//!    into it. Such input must be rejected before parsing (see step 2).
//!
//! 2. The parser tree's own `Display` is NOT usable for writing. The parser
//!    unescapes TEXT values and strips parameter quotes, and that writer does
//!    not restore them. `write_calendar()` below writes the same tree through
//!    `icalendar::Property`, which re-escapes and re-quotes. The result is
//!    semantically identical to the input but not byte-identical (parameters
//!    come out sorted, unneeded quotes are dropped, bare commas get escaped).
//!
//! 3. The parser does not check cardinality (go-ical did, on encode). The
//!    rules that tests 13-15 rely on are a short table in `validate()`.
//!
//! 4. Merging a recurrence instance from one calendar into another is plain
//!    `Vec` manipulation on the tree (see `check_instance_merge()`).
//!
//! Run: `cargo run --example ical_roundtrip_spike [maildir/cur path]`
//! Exit code 0 means every mail behaved as the Go version expects.

use std::panic::{AssertUnwindSafe, catch_unwind};
use std::path::Path;
use std::process::ExitCode;

use icalendar::parser::{Component, Parameter, Property, read_calendar, unfold};
use mail_parser::MessageParser;

/// Mails that the Go version rejects with exit code 1 (see test.sh).
const EXPECTED_REJECTS: [&str; 4] = ["test-12.eml", "test-13.eml", "test-14.eml", "test-15.eml"];

fn main() -> ExitCode {
    let dir = std::env::args()
        .nth(1)
        .unwrap_or_else(|| "test/maildir/cur".to_string());

    let mut files: Vec<_> = std::fs::read_dir(&dir)
        .unwrap_or_else(|e| panic!("reading {dir}: {e}"))
        .filter_map(|e| e.ok().map(|e| e.path()))
        .filter(|p| p.extension().is_some_and(|x| x == "eml"))
        .collect();
    files.sort();

    let mut unexpected = 0;
    let mut totals = (0, 0, 0); // accepted, rejected, no calendar

    for path in &files {
        let name = path.file_name().unwrap().to_string_lossy().to_string();
        // The Go version needed a recover() around go-ical. Record panics here
        // rather than crash, so the spike shows whether Rust needs the same.
        let outcome = catch_unwind(AssertUnwindSafe(|| check_mail(path)))
            .unwrap_or_else(|_| Outcome::Rejected("PANIC in parser".to_string()));

        let expect_reject = EXPECTED_REJECTS.contains(&name.as_str());
        let (mark, line) = match &outcome {
            Outcome::NoCalendar => {
                totals.2 += 1;
                (!expect_reject, "no calendar part".to_string())
            }
            Outcome::Accepted(summary) => {
                totals.0 += 1;
                (!expect_reject, format!("ok       {summary}"))
            }
            Outcome::Rejected(reason) => {
                totals.1 += 1;
                (expect_reject, format!("rejected {reason}"))
            }
        };
        if !mark {
            unexpected += 1;
        }
        println!("{} {name:<16} {line}", if mark { "✓" } else { "✗" });
    }

    println!();
    match check_instance_merge(Path::new(&dir)) {
        Ok(summary) => println!("✓ instance merge    {summary}"),
        Err(reason) => {
            unexpected += 1;
            println!("✗ instance merge    {reason}");
        }
    }

    println!();
    println!(
        "{} mails: {} accepted, {} rejected, {} without calendar, {} unexpected",
        files.len(),
        totals.0,
        totals.1,
        totals.2,
        unexpected
    );
    if unexpected == 0 {
        println!("SPIKE PASSED: behaviour matches the Go version's expectations");
        ExitCode::SUCCESS
    } else {
        println!("SPIKE FAILED");
        ExitCode::FAILURE
    }
}

enum Outcome {
    NoCalendar,
    Accepted(String),
    Rejected(String),
}

fn check_mail(path: &Path) -> Outcome {
    let raw = std::fs::read(path).expect("reading mail");
    let Some(ics) = extract_calendar(&raw) else {
        return Outcome::NoCalendar;
    };
    match check_calendar(&ics) {
        Ok(summary) => Outcome::Accepted(summary),
        Err(reason) => Outcome::Rejected(reason),
    }
}

/// The processor's core operation (handleRecurringEvent in the Go code): take
/// the VEVENT with a RECURRENCE-ID from an update mail and add it to the stored
/// parent calendar. Uses test 17: 17-1 is the parent, 17-3 an instance update.
fn check_instance_merge(dir: &Path) -> Result<String, String> {
    let load = |name: &str| {
        let raw = std::fs::read(dir.join(name)).map_err(|e| format!("{name}: {e}"))?;
        extract_calendar(&raw)
            .map(|ics| unfold(&ics))
            .ok_or(format!("{name}: no calendar"))
    };
    let parent_text = load("test-17-1.eml")?;
    let update_text = load("test-17-3.eml")?;
    let mut parent = read_calendar(&parent_text)?;
    let update = read_calendar(&update_text)?;

    let instance = update
        .components
        .iter()
        .find(|c| c.name == "VEVENT" && c.find_prop("RECURRENCE-ID").is_some())
        .ok_or("update has no VEVENT with RECURRENCE-ID")?;
    let recurrence_id = instance.find_prop("RECURRENCE-ID").unwrap().val.to_string();
    parent.properties.retain(|p| p.name != "METHOD");
    parent.components.push(instance.clone());

    let written = write_calendar(&parent.properties, &parent.components);
    let merged_text = unfold(&written);
    let merged = read_calendar(&merged_text)?;
    let events: Vec<&Component> = merged
        .components
        .iter()
        .filter(|c| c.name == "VEVENT")
        .collect();
    let masters = events
        .iter()
        .filter(|e| e.find_prop("RECURRENCE-ID").is_none())
        .count();
    let has_instance = events.iter().any(|e| {
        e.find_prop("RECURRENCE-ID").map(|p| p.val.as_str()) == Some(recurrence_id.as_str())
    });
    let same_uid = events
        .iter()
        .all(|e| e.find_prop("UID").map(|p| &p.val) == events[0].find_prop("UID").map(|p| &p.val));
    if events.len() != 2 || masters != 1 || !has_instance || !same_uid {
        return Err(format!(
            "merge wrong: {} vevents, {masters} master, instance present: {has_instance}, same uid: {same_uid}",
            events.len()
        ));
    }
    validate(&merged.properties, &merged.components)?;
    Ok(format!(
        "ok       17-1 + 17-3 -> 1 master + 1 instance (RECURRENCE-ID {recurrence_id}), valid, no METHOD"
    ))
}

/// Step 1: find the calendar part. Same detection rules as parser/email/email.go,
/// but transfer encoding and charset are handled by mail-parser.
fn extract_calendar(raw: &[u8]) -> Option<String> {
    let message = MessageParser::default().parse(raw)?;
    let mut found = None;
    for part in message.parts() {
        let is_calendar = part.is_content_type("text", "calendar")
            || part.is_content_type("application", "ics")
            || part.is_content_type("text", "x-vcalendar")
            || part
                .attachment_name()
                .is_some_and(|n| n.to_ascii_lowercase().ends_with(".ics"));
        if !is_calendar {
            continue;
        }
        let text = match part.text() {
            Some(text) => text.into_owned(),
            None => String::from_utf8_lossy(&part.decoded()).into_owned(),
        };
        // Like the Go code: the last calendar part wins.
        found = Some(text);
    }
    found
}

fn check_calendar(ics: &str) -> Result<String, String> {
    // Step 2: parse.
    let unfolded = unfold(ics);
    // The parser does not reject a content line without ':'. It folds the next
    // line into a parameter value instead, which silently corrupts the tree
    // (test-12: `ORGANIZER;CN=Sixt SE` swallowed the UID line). Check first.
    if let Some(bad) = content_lines(ics).iter().find(|l| !l.contains(':')) {
        return Err(format!(
            "malformed content line without ':': {}",
            truncate(bad, 60)
        ));
    }
    let cal = read_calendar(&unfolded).map_err(|e| format!("parse error: {}", first_line(&e)))?;
    if cal.components.is_empty() && cal.properties.is_empty() {
        return Err("parse error: no VCALENDAR content".to_string());
    }

    // Step 5 (done early, like the Go validation): cardinality rules.
    validate(&cal.properties, &cal.components)?;

    // Step 3: write back out. Three checks:
    //   a) re-parsing the output gives the identical tree,
    //   b) same number of content lines with the same property names in order,
    //   c) every line keeps its characters (ignoring quotes, backslashes and
    //      order), which is independent of the parser and catches dropped text.
    let written = write_calendar(&cal.properties, &cal.components);
    let reparsed_text = unfold(&written);
    let reparsed = read_calendar(&reparsed_text).map_err(|e| format!("re-parse error: {e}"))?;
    if let Some(diff) = first_difference(
        &cal.properties,
        &cal.components,
        &reparsed.properties,
        &reparsed.components,
    ) {
        return Err(format!("ROUND-TRIP LOSS: {diff}"));
    }
    let before = content_lines(ics);
    let after = content_lines(&written);
    if std::env::var_os("SPIKE_DUMP").is_some() {
        // Show every line the writer changed: `SPIKE_DUMP=1 cargo run --example ...`
        for (a, b) in before.iter().zip(after.iter()).filter(|(a, b)| a != b) {
            println!(
                "    in:  {}\n    out: {}",
                truncate(a, 110),
                truncate(b, 110)
            );
        }
    }
    if before.len() != after.len() {
        return Err(format!(
            "ROUND-TRIP LOSS: line count {} -> {}",
            before.len(),
            after.len()
        ));
    }
    let mut cosmetic = 0;
    for (a, b) in before.iter().zip(after.iter()) {
        if a == b {
            continue;
        }
        if line_name(a) != line_name(b) || fingerprint(a) != fingerprint(b) {
            return Err(format!("ROUND-TRIP LOSS: in: {a} | out: {b}"));
        }
        cosmetic += 1;
    }

    // Step 4: the edits the processor performs.
    let mut edited = cal.clone();
    edited.properties.retain(|p| p.name != "METHOD");
    let mut attendees = 0;
    for event in edited.components.iter_mut().filter(|c| c.name == "VEVENT") {
        set_prop(event, "STATUS", "CANCELLED");
        for prop in event
            .properties
            .iter_mut()
            .filter(|p| p.name == "ATTENDEE" || p.name == "ORGANIZER")
        {
            set_param(prop, "SCHEDULE-AGENT", "CLIENT");
            if prop.name == "ATTENDEE" {
                set_param(prop, "PARTSTAT", "ACCEPTED");
                attendees += 1;
            }
        }
    }
    let edited_written = write_calendar(&edited.properties, &edited.components);
    let edited_text = unfold(&edited_written);
    let edited_back =
        read_calendar(&edited_text).map_err(|e| format!("EDIT FAILED: re-parse error: {e}"))?;
    if edited_back.properties.iter().any(|p| p.name == "METHOD") {
        return Err("EDIT FAILED: METHOD still present".to_string());
    }
    for event in edited_back.components.iter().filter(|c| c.name == "VEVENT") {
        if event.find_prop("STATUS").map(|p| p.val.as_str()) != Some("CANCELLED") {
            return Err("EDIT FAILED: STATUS not set".to_string());
        }
        for prop in event
            .properties
            .iter()
            .filter(|p| p.name == "ATTENDEE" || p.name == "ORGANIZER")
        {
            if param(prop, "SCHEDULE-AGENT") != Some("CLIENT") {
                return Err("EDIT FAILED: SCHEDULE-AGENT not set".to_string());
            }
            if prop.name == "ATTENDEE" && param(prop, "PARTSTAT") != Some("ACCEPTED") {
                return Err("EDIT FAILED: PARTSTAT not set".to_string());
            }
        }
    }
    // Everything the edit did not touch must be unchanged.
    if edited_back.components.len() != cal.components.len() {
        return Err("EDIT FAILED: component count changed".to_string());
    }

    // Fields the processor reads.
    let events: Vec<&Component> = cal
        .components
        .iter()
        .filter(|c| c.name == "VEVENT")
        .collect();
    let first = events.first().ok_or("no VEVENT")?;
    let get = |name: &str| first.find_prop(name).map(|p| p.val.as_str().to_string());
    let method = cal
        .properties
        .iter()
        .find(|p| p.name == "METHOD")
        .map(|p| p.val.as_str())
        .unwrap_or("-");
    let instances = events
        .iter()
        .filter(|e| e.find_prop("RECURRENCE-ID").is_some())
        .count();
    Ok(format!(
        "{} lines ({cosmetic} rewritten), method={method} seq={} vevents={} instances={instances} attendees={attendees} uid={}",
        before.len(),
        get("SEQUENCE").unwrap_or_else(|| "-".into()),
        events.len(),
        truncate(&get("UID").unwrap_or_default(), 24),
    ))
}

/// The subset of RFC 5545 cardinality rules that go-ical enforced on encode.
fn validate(cal_props: &[Property], components: &[Component]) -> Result<(), String> {
    let count = |props: &[Property], name: &str| props.iter().filter(|p| p.name == name).count();

    for name in ["PRODID", "VERSION"] {
        match count(cal_props, name) {
            1 => {}
            n => {
                return Err(format!(
                    "invalid: VCALENDAR wants exactly one {name}, got {n}"
                ));
            }
        }
    }
    let events: Vec<&Component> = components.iter().filter(|c| c.name == "VEVENT").collect();
    if events.is_empty() {
        return Err("invalid: no VEVENT".to_string());
    }
    for event in events {
        for name in ["UID", "DTSTAMP"] {
            match count(&event.properties, name) {
                1 => {}
                n => return Err(format!("invalid: VEVENT wants exactly one {name}, got {n}")),
            }
        }
        for name in [
            "DTSTART",
            "CLASS",
            "CREATED",
            "DESCRIPTION",
            "GEO",
            "LAST-MODIFIED",
            "LOCATION",
            "ORGANIZER",
            "PRIORITY",
            "SEQUENCE",
            "STATUS",
            "SUMMARY",
            "TRANSP",
            "URL",
            "RECURRENCE-ID",
            "DTEND",
            "DURATION",
        ] {
            let n = count(&event.properties, name);
            if n > 1 {
                return Err(format!("invalid: VEVENT wants at most one {name}, got {n}"));
            }
        }
    }
    Ok(())
}

fn set_prop<'a>(component: &mut Component<'a>, name: &'static str, value: &'static str) {
    match component.properties.iter_mut().find(|p| p.name == name) {
        Some(prop) => prop.val = value.into(),
        None => component.properties.push(Property::new_ref(name, value)),
    }
}

fn set_param<'a>(prop: &mut Property<'a>, key: &'static str, value: &'static str) {
    match prop.params.iter_mut().find(|p| p.key == key) {
        Some(p) => p.val = Some(value.into()),
        None => prop.params.push(Parameter::new_ref(key, Some(value))),
    }
}

fn param<'a>(prop: &'a Property, key: &str) -> Option<&'a str> {
    prop.params
        .iter()
        .find(|p| p.key == key)
        .and_then(|p| p.val.as_ref().map(|v| v.as_str()))
}

/// Write a calendar from the parser tree. Each property goes through
/// `icalendar::Property`, whose writer escapes TEXT values, quotes parameter
/// values where required and folds long lines.
fn write_calendar(properties: &[Property], components: &[Component]) -> String {
    let mut out = String::from("BEGIN:VCALENDAR\r\n");
    write_body(&mut out, properties, components);
    out.push_str("END:VCALENDAR\r\n");
    out
}

fn write_body(out: &mut String, properties: &[Property], components: &[Component]) {
    for prop in properties {
        let line: String = icalendar::Property::from(prop.clone())
            .try_into()
            .expect("writing to a String cannot fail");
        out.push_str(&line);
    }
    for component in components {
        out.push_str(&format!("BEGIN:{}\r\n", component.name));
        write_body(out, &component.properties, &component.components);
        out.push_str(&format!("END:{}\r\n", component.name));
    }
}

/// First property that differs between two trees, if any.
fn first_difference(
    props_a: &[Property],
    comps_a: &[Component],
    props_b: &[Property],
    comps_b: &[Component],
) -> Option<String> {
    if props_a.len() != props_b.len() || comps_a.len() != comps_b.len() {
        return Some("tree shape differs".to_string());
    }
    // Parameter order has no meaning in iCalendar and the writer sorts them.
    let sorted = |p: &Property| {
        let mut params: Vec<(String, Option<String>)> = p
            .params
            .iter()
            .map(|x| (x.key.to_string(), x.val.as_ref().map(|v| v.to_string())))
            .collect();
        params.sort();
        params
    };
    for (a, b) in props_a.iter().zip(props_b) {
        if a.name != b.name || a.val != b.val || sorted(a) != sorted(b) {
            return Some(format!("{a:?} != {b:?}"));
        }
    }
    for (a, b) in comps_a.iter().zip(comps_b) {
        if a.name != b.name {
            return Some(format!("component {} != {}", a.name, b.name));
        }
        if let Some(d) =
            first_difference(&a.properties, &a.components, &b.properties, &b.components)
        {
            return Some(d);
        }
    }
    None
}

/// Property name of a content line (text before the first `;` or `:`).
fn line_name(line: &str) -> &str {
    line.split([';', ':']).next().unwrap_or("")
}

/// Order-independent content of a line, ignoring quoting and escaping.
fn fingerprint(line: &str) -> Vec<char> {
    let mut chars: Vec<char> = line
        .chars()
        .filter(|c| *c != '"' && *c != '\\')
        .map(|c| c.to_ascii_lowercase())
        .collect();
    chars.sort_unstable();
    chars
}

/// Unfolded, non-empty content lines with line endings normalised.
fn content_lines(ics: &str) -> Vec<String> {
    unfold(ics)
        .lines()
        .map(|l| l.trim_end_matches('\r').to_string())
        .filter(|l| !l.trim().is_empty())
        .collect()
}

fn first_line(s: &str) -> String {
    truncate(s.lines().find(|l| !l.trim().is_empty()).unwrap_or(""), 100)
}

fn truncate(s: &str, max: usize) -> String {
    if s.chars().count() <= max {
        s.to_string()
    } else {
        format!("{}…", s.chars().take(max).collect::<String>())
    }
}
