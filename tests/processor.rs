//! Integration tests for the processor, using the mails in `test/maildir/cur`
//! and in-memory storage. Each numbered test is documented in `test/test-N.md`.

use calmailproc::ical::{self, Calendar, Component, Event};
use calmailproc::processor::Processor;
use calmailproc::storage::{MemoryStorage, Storage};

type TestProcessor = Processor<MemoryStorage>;

fn processor(process_replies: bool) -> TestProcessor {
    Processor::new(MemoryStorage::new(), process_replies)
}

fn mail(name: &str) -> Vec<u8> {
    std::fs::read(format!("test/maildir/cur/{name}")).unwrap_or_else(|e| panic!("{name}: {e}"))
}

/// Processes a test mail that must succeed and returns the status message.
fn process(processor: &mut TestProcessor, name: &str) -> String {
    processor
        .process_email(&mail(name))
        .unwrap_or_else(|failure| panic!("{name}: {} ({})", failure.status, failure.error))
}

/// The one event that must be in storage.
fn only_event(processor: &TestProcessor) -> Event {
    let events = processor.storage.list_events();
    assert_eq!(events.len(), 1, "expected exactly one stored event");
    assert_eq!(processor.storage.event_count(), 1);
    events[0].clone()
}

fn calendar(event: &Event) -> Calendar {
    ical::decode(&event.raw_data).expect("stored event must be valid iCalendar")
}

fn first_vevent(event: &Event) -> Component {
    calendar(event).events().next().expect("VEVENT").clone()
}

fn value(component: &Component, name: &str) -> String {
    component.value(name).unwrap_or_default().to_string()
}

/// The PARTSTAT of the attendee with this address in the first VEVENT.
fn partstat(event: &Event, address: &str) -> Option<String> {
    first_vevent(event)
        .props("ATTENDEE")
        .find(|a| a.value.eq_ignore_ascii_case(&format!("mailto:{address}")))
        .map(|a| a.param("PARTSTAT").unwrap_or("NEEDS-ACTION").to_string())
}

fn occurrences(event: &Event) -> Vec<Component> {
    calendar(event)
        .events()
        .filter(|e| e.has_prop("RECURRENCE-ID"))
        .cloned()
        .collect()
}

fn parents(event: &Event) -> Vec<Component> {
    calendar(event)
        .events()
        .filter(|e| !e.has_prop("RECURRENCE-ID"))
        .cloned()
        .collect()
}

#[test]
fn test_00_no_calendar_data() {
    let mut processor = processor(true);
    let status = process(&mut processor, "test-0.eml");
    assert_eq!(status, "Processed E-Mail without calendar event");
    assert_eq!(processor.storage.event_count(), 0);
    assert!(processor.storage.list_events().is_empty());
}

#[test]
fn test_01_create_then_cancel() {
    let mut processor = processor(true);

    let status = process(&mut processor, "test-01-1.eml");
    assert!(status.starts_with("Stored new event with UID "), "{status}");
    let event = only_event(&processor);
    assert_eq!(event.summary, "Test Event 1");
    assert_eq!(value(&first_vevent(&event), "STATUS"), "CONFIRMED");

    let status = process(&mut processor, "test-01-2.eml");
    assert!(status.ends_with(", new sequence: 1"), "{status}");
    let event = only_event(&processor);
    assert!(
        [
            "Test Event 1",
            "Cancelled: Test Event 1",
            "Abgesagt: Test Event 1"
        ]
        .contains(&event.summary.as_str()),
        "unexpected summary after cancellation: {}",
        event.summary
    );
    assert_eq!(value(&first_vevent(&event), "STATUS"), "CANCELLED");
}

#[test]
fn test_02_update_date_time() {
    let mut processor = processor(true);

    process(&mut processor, "test-02-1.eml");
    let initial = only_event(&processor);
    assert_eq!(initial.summary, "Test 2");
    let initial = first_vevent(&initial);

    let status = process(&mut processor, "test-02-2.eml");
    assert!(status.starts_with("Updated event with UID "), "{status}");
    let updated = first_vevent(&only_event(&processor));

    assert_ne!(value(&initial, "DTSTART"), value(&updated, "DTSTART"));
    assert_ne!(value(&initial, "DTEND"), value(&updated, "DTEND"));
    assert_eq!(value(&updated, "SEQUENCE"), "1");
}

#[test]
fn test_03_recurring_events() {
    let mut processor = processor(true);
    let uid = "040000008200E00074C5B7101A82E0080000000044440AFCBB91DB0100000000000000001000000087598F58784D4541BAA76F1829CFE9A1";

    process(&mut processor, "test-03-1.eml");
    let recurring = only_event(&processor);
    assert_eq!(recurring.uid, uid);
    assert!(first_vevent(&recurring).has_prop("RRULE"));

    // Cancellation of one occurrence, then a modification of one occurrence.
    for name in ["test-03-2.eml", "test-03-3.eml"] {
        let status = process(&mut processor, name);
        assert_eq!(
            status,
            format!("Updated recurring event instance with UID {uid}")
        );
        assert_eq!(processor.storage.event_count(), 1);
    }

    let event = only_event(&processor);
    assert_eq!(parents(&event).len(), 1);
    assert!(parents(&event)[0].has_prop("RRULE"));
    assert!(
        !occurrences(&event).is_empty(),
        "occurrence updates must be stored"
    );
}

#[test]
fn test_04_duplicate_events() {
    let mut processor = processor(true);

    process(&mut processor, "test-04-1.eml");
    let first = only_event(&processor);
    assert_eq!(first.summary, "Test 4");

    let status = process(&mut processor, "test-04-2.eml");
    assert!(status.ends_with(", new sequence: 0"), "{status}");
    let second = only_event(&processor);
    assert_eq!(second.uid, first.uid);
    assert_eq!(second.summary, "Test 4");

    for address in [
        "markus.brechtel@uk-koeln.de",
        "brechtel@med.uni-frankfurt.de",
    ] {
        assert!(
            partstat(&second, address).is_some(),
            "attendee {address} missing"
        );
    }
}

/// An invitation followed by a reply. `reply_status` is the PARTSTAT the
/// replying attendee must end up with.
fn invitation_and_reply(invitation: &str, reply: &str, summary: &str, reply_status: &str) {
    let responding = "markus.brechtel@uk-koeln.de";
    let organizer = "markus.brechtel@thengo.net";

    let mut processor = processor(true);
    process(&mut processor, invitation);
    let invited = only_event(&processor);
    assert_eq!(invited.summary, summary);
    assert_eq!(
        partstat(&invited, responding).as_deref(),
        Some("NEEDS-ACTION")
    );

    let status = process(&mut processor, reply);
    assert_eq!(
        status,
        format!("Updated attendee status for event with UID {}", invited.uid)
    );
    let updated = only_event(&processor);
    assert_eq!(
        partstat(&updated, responding).as_deref(),
        Some(reply_status)
    );
    // The invitation itself must survive the reply: same summary, the other
    // attendee untouched, and nothing but the PARTSTAT changed.
    assert_eq!(updated.summary, summary);
    assert_eq!(value(&first_vevent(&updated), "SUMMARY"), summary);
    assert_eq!(
        partstat(&updated, organizer).as_deref(),
        Some("NEEDS-ACTION")
    );
    assert_eq!(
        first_vevent(&updated).properties.len(),
        first_vevent(&invited).properties.len()
    );

    // With reply processing switched off the reply is ignored.
    let mut ignoring = self::processor(false);
    process(&mut ignoring, invitation);
    let status = process(&mut ignoring, reply);
    assert_eq!(status, "Ignoring calendar REPLY method as configured");
    assert_eq!(
        partstat(&only_event(&ignoring), responding).as_deref(),
        Some("NEEDS-ACTION")
    );
}

#[test]
fn test_05_invitation_and_response() {
    invitation_and_reply("test-05-1.eml", "test-05-2.eml", "Test 5", "ACCEPTED");
}

#[test]
fn test_06_invitation_and_decline() {
    invitation_and_reply("test-06-1.eml", "test-06-2.eml", "Test 6", "DECLINED");
}

#[test]
fn reply_without_stored_event_is_stored() {
    let mut processor = processor(true);
    let status = process(&mut processor, "test-05-2.eml");
    assert!(
        status.starts_with("Stored new reply event with UID "),
        "{status}"
    );
    assert_eq!(processor.storage.event_count(), 1);
}

#[test]
fn test_07_sequential_updates() {
    let mut processor = processor(true);

    process(&mut processor, "test-07-1.eml");
    let original = first_vevent(&only_event(&processor));
    assert_eq!(value(&original, "SEQUENCE"), "0");

    // Updates arrive as sequence 3, 4, 2, 1: the last two are outdated.
    let status = process(&mut processor, "test-07-2.eml");
    assert!(status.ends_with(", new sequence: 3"), "{status}");
    let status = process(&mut processor, "test-07-3.eml");
    assert!(status.ends_with(", new sequence: 4"), "{status}");
    let status = process(&mut processor, "test-07-4.eml");
    assert!(
        status.starts_with("Not processing older event (sequence: 2 vs 4, DTSTAMP comparison)"),
        "{status}"
    );
    let status = process(&mut processor, "test-07-5.eml");
    assert!(
        status.starts_with("Not processing older event (sequence: 1 vs 4, DTSTAMP comparison)"),
        "{status}"
    );

    let last = first_vevent(&only_event(&processor));
    assert_eq!(value(&last, "SEQUENCE"), "4");
    assert_ne!(value(&original, "DTSTART"), value(&last, "DTSTART"));
    assert_ne!(value(&original, "DTEND"), value(&last, "DTEND"));

    // Out of order: the newest update first, then the original.
    let mut out_of_order = self::processor(true);
    process(&mut out_of_order, "test-07-3.eml");
    process(&mut out_of_order, "test-07-1.eml");
    assert_eq!(
        value(&first_vevent(&only_event(&out_of_order)), "SEQUENCE"),
        "4"
    );
}

#[test]
fn test_08_non_outlook_event() {
    let mut processor = processor(true);
    process(&mut processor, "test-08.eml");
    let event = only_event(&processor);
    assert_eq!(event.summary, "Test 8");
    let vevent = first_vevent(&event);
    assert_eq!(value(&vevent, "STATUS"), "CONFIRMED");
    assert_eq!(
        value(&vevent, "ORGANIZER"),
        "mailto:C.Lee@med.uni-frankfurt.de"
    );
    for address in [
        "brechtel@med.uni-frankfurt.de",
        "markus.brechtel@uk-koeln.de",
    ] {
        assert!(
            partstat(&event, address).is_some(),
            "attendee {address} missing"
        );
    }
}

#[test]
fn test_09_forwarded_calendar_invitation() {
    let mut processor = processor(true);
    process(&mut processor, "test-09.eml");
    let event = only_event(&processor);
    assert_eq!(event.summary, "Test 9");
    let vevent = first_vevent(&event);
    assert_eq!(value(&vevent, "STATUS"), "CONFIRMED");
    assert_eq!(
        value(&vevent, "ORGANIZER"),
        "mailto:C.Lee@med.uni-frankfurt.de"
    );
    assert!(partstat(&event, "ludwig.montag@med.uni-frankfurt.de").is_some());
}

#[test]
fn test_10_inline_calendar_content() {
    let mut processor = processor(true);
    process(&mut processor, "test-10.eml");
    let event = only_event(&processor);
    assert_eq!(event.summary, "Test 10");
    let vevent = first_vevent(&event);
    assert_eq!(value(&vevent, "X-MICROSOFT-CDO-BUSYSTATUS"), "TENTATIVE");
    assert_eq!(value(&vevent, "ORGANIZER"), "mailto:example@example.com");
    assert!(partstat(&event, "markus.brechtel@uk-koeln.de").is_some());
    assert!(
        vevent.components.iter().any(|c| c.name == "VALARM"),
        "the VALARM component must be kept"
    );
}

#[test]
fn test_11_out_of_sequence_mail() {
    let uid = "040000008200E00074C5B7101A82E0080000000071F706FA87AFDB010000000000000000100000004BEAA256602AD04DB92DF1261AC49A16";
    let is_cancelled = |processor: &mut TestProcessor| {
        let event = processor
            .storage
            .get_event(uid)
            .unwrap()
            .expect("event stored");
        value(&first_vevent(&event), "STATUS") == "CANCELLED"
    };

    // In order: request, then cancellation.
    let mut processor = self::processor(true);
    process(&mut processor, "test-11-2.eml");
    let status = process(&mut processor, "test-11-1.eml");
    assert!(status.ends_with(", new sequence: 0"), "{status}");
    assert!(is_cancelled(&mut processor));

    // Out of order: the cancellation arrives first and must not be undone by
    // the older request.
    let mut processor = self::processor(true);
    let status = process(&mut processor, "test-11-1.eml");
    assert!(status.starts_with("Stored new event with UID "), "{status}");
    let status = process(&mut processor, "test-11-2.eml");
    assert!(
        status.starts_with("Not processing older event (sequence: 0 vs 0, DTSTAMP comparison)"),
        "{status}"
    );
    assert!(is_cancelled(&mut processor));
}

/// A mail whose calendar must be rejected with an error mentioning `expected`.
fn rejected(name: &str, expected: &str) {
    let mut processor = processor(true);
    let failure = processor
        .process_email(&mail(name))
        .expect_err("invalid calendar data must be rejected");
    let error = failure.error.to_string();
    assert!(error.contains(expected), "{name}: {error}");
    assert_eq!(processor.storage.event_count(), 0, "nothing may be stored");
}

#[test]
fn test_12_invalid_calendar_data() {
    rejected("test-12.eml", "malformed content line without ':'");
}

#[test]
fn test_13_specification_violation() {
    rejected("test-13.eml", "want at most one URL property, got 2");
}

#[test]
fn test_14_missing_prodid() {
    rejected("test-14.eml", "want exactly one PRODID property, got 0");
}

#[test]
fn test_15_duplicate_dtstamp() {
    rejected("test-15.eml", "want exactly one DTSTAMP property, got 2");
}

#[test]
fn test_16_invalid_dtstamp_format() {
    let mut processor = processor(true);
    let status = process(&mut processor, "test-16-1.eml");
    assert_eq!(status, "Stored new event with UID bahn2023-07-01125700");
    let status = process(&mut processor, "test-16-2.eml");
    assert_eq!(
        status,
        "Updated event with UID bahn2023-07-01125700, new sequence: 0"
    );
    assert_eq!(only_event(&processor).uid, "bahn2023-07-01125700");
}

#[test]
fn test_17_recurring_instances_with_parent() {
    let uid = "040000008200E00074C5B7101A82E0080000000060FA38123DBBDB010000000000000000100000009123BEADE9978A4AA0AC92EF2005A108";
    let mut processor = processor(false);
    let stored = |processor: &mut TestProcessor| processor.storage.get_event(uid).unwrap().unwrap();

    // 17-0 is itself an occurrence update and arrives first.
    let status = process(&mut processor, "test-17-0.eml");
    assert!(status.starts_with("Stored new event with UID"), "{status}");
    assert_eq!(stored(&mut processor).sequence, 7);

    for (name, expected) in [
        (
            "test-17-1.eml",
            "Updated parent event while preserving instances",
        ),
        ("test-17-2.eml", "Updated recurring event instance"),
        (
            "test-17-7.eml",
            "Updated parent event while preserving instances",
        ),
    ] {
        let status = process(&mut processor, name);
        assert!(status.starts_with(expected), "{name}: {status}");
    }
    assert_eq!(
        stored(&mut processor).sequence,
        5,
        "parent sequence from test-17-7"
    );

    for (name, recurrence_id) in [
        ("test-17-3.eml", "20250509T150000"),
        ("test-17-4.eml", "20250523T150000"), // cancelled
        ("test-17-5.eml", "20250606T150000"), // cancelled
        ("test-17-6.eml", "20250509T150000"), // replaces the one from 17-3
    ] {
        let status = process(&mut processor, name);
        assert!(
            status.starts_with("Updated recurring event instance"),
            "{name}: {status}"
        );
        let event = stored(&mut processor);
        assert!(
            occurrences(&event)
                .iter()
                .any(|o| value(o, "RECURRENCE-ID") == recurrence_id),
            "{name}: occurrence {recurrence_id} not stored"
        );
    }

    let event = stored(&mut processor);
    let parents = parents(&event);
    assert_eq!(parents.len(), 1, "exactly one parent event");
    assert!(parents[0].has_prop("RRULE"));
    assert_eq!(value(&parents[0], "SEQUENCE"), "5");

    let occurrences = occurrences(&event);
    assert!(
        occurrences.len() >= 3,
        "found only {} occurrences",
        occurrences.len()
    );
    let may_9: Vec<_> = occurrences
        .iter()
        .filter(|o| value(o, "RECURRENCE-ID") == "20250509T150000")
        .collect();
    assert_eq!(may_9.len(), 1, "an occurrence is replaced, not duplicated");
    assert_eq!(value(may_9[0], "SEQUENCE"), "6");
    for cancelled in ["20250523T150000", "20250606T150000"] {
        let occurrence = occurrences
            .iter()
            .find(|o| value(o, "RECURRENCE-ID") == cancelled)
            .unwrap();
        assert_eq!(value(occurrence, "STATUS"), "CANCELLED");
    }
    // The time zone definition must survive all the merging.
    assert!(
        calendar(&event)
            .components
            .iter()
            .any(|c| c.name == "VTIMEZONE")
    );
}

#[test]
fn test_18_instance_before_parent() {
    let uid = "synthetic-test18-event@example.com";
    let mut processor = processor(false);

    let status = process(&mut processor, "test-18-0.eml");
    assert!(status.starts_with("Stored new event"), "{status}");
    let event = processor.storage.get_event(uid).unwrap().unwrap();
    assert_eq!(event.sequence, 2);
    assert_eq!(occurrences(&event).len(), 1);

    let status = process(&mut processor, "test-18-1.eml");
    assert!(
        status.starts_with("Updated parent event while preserving instances"),
        "{status}"
    );
    let event = processor.storage.get_event(uid).unwrap().unwrap();
    let parents = parents(&event);
    assert_eq!(parents.len(), 1);
    assert!(parents[0].has_prop("RRULE"));
    assert_eq!(value(&parents[0], "SUMMARY"), "Test 18 - Weekly Meeting");
    assert_eq!(occurrences(&event).len(), 1);
}

#[test]
fn stored_events_have_no_method() {
    for name in [
        "test-01-1.eml",
        "test-03-2.eml",
        "test-05-2.eml",
        "test-16-1.eml",
    ] {
        let mut processor = processor(true);
        process(&mut processor, name);
        let event = only_event(&processor);
        assert!(
            !calendar(&event).has_prop("METHOD"),
            "{name}: METHOD was stored"
        );
        ical::validate_event(&event.raw_data).unwrap();
    }
}

/// Every ORGANIZER and ATTENDEE of every VEVENT, with its SCHEDULE-AGENT.
fn schedule_agents(event: &Event) -> Vec<(String, Option<String>)> {
    calendar(event)
        .events()
        .flat_map(|vevent| vevent.properties.clone())
        .filter(|p| p.name == "ORGANIZER" || p.name == "ATTENDEE")
        .map(|p| {
            (
                format!("{} {}", p.name, p.value),
                p.param("SCHEDULE-AGENT").map(str::to_string),
            )
        })
        .collect()
}

fn assert_client_scheduling(event: &Event, expected_participants: usize, context: &str) {
    let agents = schedule_agents(event);
    assert_eq!(agents.len(), expected_participants, "{context}: {agents:?}");
    for (participant, agent) in agents {
        assert_eq!(
            agent.as_deref(),
            Some("CLIENT"),
            "{context}: {participant} must have SCHEDULE-AGENT=CLIENT"
        );
    }
}

#[test]
fn stored_events_tell_the_server_not_to_schedule() {
    // issues/bug-missing-schedule-agent-client.md: without SCHEDULE-AGENT=CLIENT
    // a CalDAV server with scheduling mails the participants again.

    // New event: organizer and both attendees.
    let mut processor = processor(true);
    process(&mut processor, "test-05-1.eml");
    let invited = only_event(&processor);
    assert_client_scheduling(&invited, 3, "new event");

    // The other parameters are untouched.
    let attendee = first_vevent(&invited)
        .props("ATTENDEE")
        .find(|a| a.value == "mailto:markus.brechtel@uk-koeln.de")
        .cloned()
        .unwrap();
    for (key, value) in [
        ("RSVP", "TRUE"),
        ("PARTSTAT", "NEEDS-ACTION"),
        ("CUTYPE", "INDIVIDUAL"),
        ("ROLE", "REQ-PARTICIPANT"),
    ] {
        assert_eq!(attendee.param(key), Some(value), "{key}");
    }
    assert_eq!(attendee.params.len(), 5, "only SCHEDULE-AGENT was added");

    // After a reply updated the attendee status.
    process(&mut processor, "test-05-2.eml");
    let replied = only_event(&processor);
    assert_client_scheduling(&replied, 3, "after reply");
    assert_eq!(
        partstat(&replied, "markus.brechtel@uk-koeln.de").as_deref(),
        Some("ACCEPTED")
    );

    // After an update and after a cancellation.
    let mut processor = self::processor(true);
    process(&mut processor, "test-02-1.eml");
    process(&mut processor, "test-02-2.eml");
    assert_client_scheduling(&only_event(&processor), 2, "after update");
    let mut processor = self::processor(true);
    process(&mut processor, "test-01-1.eml");
    process(&mut processor, "test-01-2.eml");
    assert_client_scheduling(&only_event(&processor), 2, "after cancellation");

    // Recurring event: the parent and every merged occurrence.
    let mut processor = self::processor(true);
    for name in [
        "test-17-0.eml",
        "test-17-1.eml",
        "test-17-2.eml",
        "test-17-7.eml",
        "test-17-3.eml",
        "test-17-4.eml",
        "test-17-5.eml",
        "test-17-6.eml",
    ] {
        process(&mut processor, name);
    }
    let recurring = only_event(&processor);
    let vevents = parents(&recurring).len() + occurrences(&recurring).len();
    assert!(vevents >= 4, "parent and occurrences: {vevents}");
    assert_client_scheduling(&recurring, vevents * 2, "recurring event");
}

#[test]
fn incoming_schedule_agent_server_is_overwritten() {
    let mut processor = processor(true);
    let mail = String::from_utf8(parent_mail("schedule-agent-server", 0))
        .unwrap()
        .replace(
            "ORGANIZER:organizer@example.com",
            "ORGANIZER;SCHEDULE-AGENT=SERVER:mailto:organizer@example.com\nATTENDEE;schedule-agent=NONE;CN=A:mailto:a@example.com",
        );
    process_raw(&mut processor, mail.as_bytes());
    let event = only_event(&processor);
    assert_client_scheduling(&event, 2, "overwritten");
    let attendee = first_vevent(&event).prop("ATTENDEE").cloned().unwrap();
    assert_eq!(
        attendee.params.len(),
        2,
        "no duplicate parameter: {:?}",
        attendee.params
    );
}

// --- Synthetic recurring event mails -------------------------------------

/// A multipart mail with one calendar part holding a single VEVENT.
fn synthetic_mail(vevent_lines: &[&str]) -> Vec<u8> {
    format!(
        "From: organizer@example.com\nTo: attendee@example.com\nSubject: Meeting\nMIME-Version: 1.0\nContent-Type: multipart/mixed; boundary=\"boundary\"\n\n--boundary\nContent-Type: text/plain\n\nHello.\n\n--boundary\nContent-Type: text/calendar; method=REQUEST; charset=UTF-8\nContent-Transfer-Encoding: 7bit\n\nBEGIN:VCALENDAR\nPRODID:-//Test//EN\nVERSION:2.0\nMETHOD:REQUEST\nBEGIN:VEVENT\n{}\nORGANIZER:organizer@example.com\nEND:VEVENT\nEND:VCALENDAR\n--boundary--\n",
        vevent_lines.join("\n")
    )
    .into_bytes()
}

fn parent_mail(uid: &str, sequence: u32) -> Vec<u8> {
    synthetic_mail(&[
        "SUMMARY:Weekly Meeting",
        "DTSTART:20250505T100000Z",
        "DTEND:20250505T110000Z",
        &format!("UID:{uid}"),
        &format!("SEQUENCE:{sequence}"),
        "RRULE:FREQ=WEEKLY;COUNT=5",
        "DTSTAMP:20250501T120000Z",
    ])
}

fn occurrence_mail(uid: &str, day: &str, sequence: u32) -> Vec<u8> {
    synthetic_mail(&[
        &format!("SUMMARY:Weekly Meeting (moved, {day})"),
        &format!("DTSTART:{day}T140000Z"),
        &format!("DTEND:{day}T150000Z"),
        &format!("UID:{uid}"),
        &format!("SEQUENCE:{sequence}"),
        &format!("RECURRENCE-ID:{day}T100000Z"),
        "DTSTAMP:20250430T120000Z",
    ])
}

fn process_raw(processor: &mut TestProcessor, raw: &[u8]) -> String {
    processor
        .process_email(raw)
        .unwrap_or_else(|failure| panic!("{} ({})", failure.status, failure.error))
}

#[test]
fn recurring_event_with_occurrence_update() {
    let uid = "recurring-event-1";
    let mut processor = processor(true);
    process_raw(&mut processor, &parent_mail(uid, 0));
    process_raw(&mut processor, &occurrence_mail(uid, "20250512", 1));

    let event = only_event(&processor);
    assert_eq!(parents(&event).len(), 1);
    assert_eq!(value(&parents(&event)[0], "DTSTART"), "20250505T100000Z");
    let occurrences = occurrences(&event);
    assert_eq!(occurrences.len(), 1);
    assert_eq!(value(&occurrences[0], "RECURRENCE-ID"), "20250512T100000Z");
    assert_eq!(value(&occurrences[0], "DTSTART"), "20250512T140000Z");
    assert_eq!(
        value(&occurrences[0], "SUMMARY"),
        "Weekly Meeting (moved, 20250512)"
    );
}

#[test]
fn occurrence_updates_are_not_skipped_for_a_newer_parent() {
    // The parent has sequence 7; occurrence updates with lower sequence
    // numbers must still be merged.
    let uid = "test-recurring-event-issue17";
    let mut processor = processor(false);
    let status = process_raw(&mut processor, &parent_mail(uid, 7));
    assert_eq!(status, format!("Stored new event with UID {uid}"));

    for (day, sequence) in [("20250512", 0), ("20250519", 2), ("20250526", 3)] {
        let status = process_raw(&mut processor, &occurrence_mail(uid, day, sequence));
        assert_eq!(
            status,
            format!("Updated recurring event instance with UID {uid}")
        );
    }
    let event = only_event(&processor);
    assert_eq!(parents(&event).len(), 1);
    assert_eq!(occurrences(&event).len(), 3);
}

#[test]
fn occurrences_before_parent_are_preserved() {
    let uid = "test-recurring-event-reverse";
    let mut processor = processor(false);

    let status = process_raw(&mut processor, &occurrence_mail(uid, "20250512", 0));
    assert_eq!(status, format!("Stored new event with UID {uid}"));
    let status = process_raw(&mut processor, &occurrence_mail(uid, "20250519", 2));
    assert_eq!(
        status,
        format!("Updated recurring event instance with UID {uid}")
    );
    assert_eq!(occurrences(&only_event(&processor)).len(), 2);

    let status = process_raw(&mut processor, &parent_mail(uid, 7));
    assert_eq!(
        status,
        format!("Updated parent event while preserving instances with UID {uid}")
    );
    let event = only_event(&processor);
    assert_eq!(parents(&event).len(), 1);
    assert_eq!(value(&parents(&event)[0], "SEQUENCE"), "7");
    assert_eq!(occurrences(&event).len(), 2);
    // The parent comes first, so the stored event reports its sequence.
    assert_eq!(event.sequence, 7);
}

#[test]
fn occurrence_cancellation_marks_only_that_occurrence() {
    let uid = "cancel-one-occurrence";
    let mut processor = processor(true);
    process_raw(&mut processor, &parent_mail(uid, 0));
    process_raw(&mut processor, &occurrence_mail(uid, "20250512", 1));

    let cancel = String::from_utf8(occurrence_mail(uid, "20250512", 2))
        .unwrap()
        .replace("method=REQUEST", "method=CANCEL")
        .replace("METHOD:REQUEST", "METHOD:CANCEL");
    process_raw(&mut processor, cancel.as_bytes());

    let event = only_event(&processor);
    assert_eq!(value(&occurrences(&event)[0], "STATUS"), "CANCELLED");
    assert_eq!(occurrences(&event).len(), 1);
    assert!(!parents(&event)[0].has_prop("STATUS"));
}

#[test]
fn stored_event_with_several_occurrences_round_trips() {
    let mut calendar = Component::new("VCALENDAR");
    calendar.set_prop("PRODID", "-//calmailproc//Calendar//EN");
    calendar.set_prop("VERSION", "2.0");
    let vevent = |summary: &str, recurrence_id: Option<&str>| {
        let mut event = Component::new("VEVENT");
        event.set_prop("UID", "multi-recurring-event");
        event.set_prop("SUMMARY", summary);
        event.set_prop("DTSTAMP", "20230101T000000Z");
        match recurrence_id {
            Some(id) => event.set_prop("RECURRENCE-ID", id),
            None => event.set_prop("RRULE", "FREQ=WEEKLY;COUNT=4"),
        }
        event
    };
    calendar.components = vec![
        vevent("Multi-Instance Meeting", None),
        vevent("Modified Instance 1", Some("20230108T120000Z")),
        vevent("Modified Instance 2", Some("20230115T120000Z")),
    ];

    let mut store = MemoryStorage::new();
    store
        .store_event(&Event {
            uid: "multi-recurring-event".to_string(),
            raw_data: ical::encode(&calendar).unwrap(),
            summary: "Multi-Instance Meeting".to_string(),
            method: "REQUEST".to_string(),
            sequence: 1,
        })
        .unwrap();

    let event = store.get_event("multi-recurring-event").unwrap().unwrap();
    assert_eq!(parents(&event).len(), 1);
    let occurrences = occurrences(&event);
    assert_eq!(occurrences.len(), 2);
    assert_eq!(value(&occurrences[0], "SUMMARY"), "Modified Instance 1");
    assert_eq!(value(&occurrences[1], "SUMMARY"), "Modified Instance 2");
}
