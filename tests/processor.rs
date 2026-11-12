//! Integration tests for the processor, using the mails in `test/maildir/cur`
//! and in-memory storage. Each numbered test is documented in `test/test-N.md`.

use calmailproc::ical::{self, Calendar, Component, Event};
use calmailproc::processor::Processor;
use calmailproc::storage::MemoryStorage;

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
fn test_06_stores_the_comment_of_the_reply() {
    let mut processor = processor(true);
    process(&mut processor, "test-06-1.eml");
    process(&mut processor, "test-06-2.eml");
    let attendee = first_vevent(&only_event(&processor))
        .props("ATTENDEE")
        .find(|a| a.value == "mailto:markus.brechtel@uk-koeln.de")
        .cloned()
        .unwrap();
    assert_eq!(attendee.param("X-RESPONSE-COMMENT"), Some("no, sorry"));

    // A later reply without a comment clears the old one.
    let uid = only_event(&processor).uid;
    let reply = synthetic_reply(
        &uid,
        "ATTENDEE;PARTSTAT=ACCEPTED:MAILTO:markus.brechtel@uk-koeln.de",
        None,
        None,
    );
    process_raw(&mut processor, &reply);
    let event = only_event(&processor);
    assert_eq!(
        partstat(&event, "markus.brechtel@uk-koeln.de").as_deref(),
        Some("ACCEPTED")
    );
    let attendee = first_vevent(&event)
        .props("ATTENDEE")
        .find(|a| a.value == "mailto:markus.brechtel@uk-koeln.de")
        .cloned()
        .unwrap();
    assert_eq!(attendee.param("X-RESPONSE-COMMENT"), None);
}

#[test]
fn reply_for_unknown_event_is_ignored_unless_configured() {
    // A reply is not an event. Stored as one it would also keep the real
    // invitation out when that is processed later.
    let mut processor = processor(true);
    let status = process(&mut processor, "test-05-2.eml");
    assert!(
        status.starts_with("Ignoring reply for unknown event with UID "),
        "{status}"
    );
    assert_eq!(processor.storage.event_count(), 0);
    process(&mut processor, "test-05-1.eml");
    assert_eq!(only_event(&processor).summary, "Test 5");

    let mut storing = self::processor(true);
    storing.config.process_replies_without_event = true;
    let status = process(&mut storing, "test-05-2.eml");
    assert!(
        status.starts_with("Stored new reply event with UID "),
        "{status}"
    );
    assert_eq!(storing.storage.event_count(), 1);
}

#[test]
fn reply_from_unknown_attendee_adds_only_that_attendee() {
    let mut processor = processor(true);
    process(&mut processor, "test-05-1.eml");
    let invited = only_event(&processor);

    // The same reply, but from an address that was not invited.
    let reply = String::from_utf8(synthetic_reply(
        &invited.uid,
        "ATTENDEE;PARTSTAT=TENTATIVE;CN=Deputy:mailto:deputy@example.com",
        Some("COMMENT:I come \"instead\"\\nof my boss"),
        None,
    ))
    .unwrap();
    let status = process_raw(&mut processor, reply.as_bytes());
    assert_eq!(
        status,
        format!(
            "Added attendee mailto:deputy@example.com to event with UID {}",
            invited.uid
        )
    );

    let updated = only_event(&processor);
    // The invitation is intact: same summary, the two invited attendees
    // unchanged, one attendee more.
    assert_eq!(updated.summary, "Test 5");
    for address in ["markus.brechtel@uk-koeln.de", "markus.brechtel@thengo.net"] {
        assert_eq!(partstat(&updated, address).as_deref(), Some("NEEDS-ACTION"));
    }
    assert_eq!(
        partstat(&updated, "deputy@example.com").as_deref(),
        Some("TENTATIVE")
    );
    let vevent = first_vevent(&updated);
    assert_eq!(vevent.props("ATTENDEE").count(), 3);
    assert_eq!(
        vevent.properties.len(),
        first_vevent(&invited).properties.len() + 1
    );
    let deputy = vevent
        .props("ATTENDEE")
        .find(|a| a.value == "mailto:deputy@example.com")
        .unwrap();
    assert_eq!(deputy.param("CN"), Some("Deputy"));
    assert_eq!(
        deputy.param("X-RESPONSE-COMMENT"),
        Some("I come 'instead' of my boss")
    );
    // New on the server, so the server must not schedule for them.
    assert_eq!(deputy.param("SCHEDULE-AGENT"), Some("CLIENT"));
}

#[test]
fn reply_for_an_occurrence_that_is_not_stored_changes_nothing() {
    let mut processor = processor(true);
    process(&mut processor, "test-05-1.eml");
    let before = only_event(&processor);
    let reply = synthetic_reply(
        &before.uid,
        "ATTENDEE;PARTSTAT=DECLINED:mailto:markus.brechtel@uk-koeln.de",
        None,
        Some("RECURRENCE-ID:20250326T100000"),
    );
    let status = process_raw(&mut processor, &reply);
    assert!(
        status.starts_with("Ignoring reply for an occurrence that is not stored"),
        "{status}"
    );
    assert_eq!(only_event(&processor), before);
}

#[test]
fn invalid_reply_is_an_error_and_changes_nothing() {
    let mut processor = processor(true);
    process(&mut processor, "test-05-1.eml");
    let before = only_event(&processor);
    let reply = synthetic_reply(
        &before.uid,
        "ATTENDEE:mailto:someone@example.com",
        None,
        None,
    );
    let failure = processor.process_email(&reply).unwrap_err();
    assert!(
        failure.error.to_string().contains("no PARTSTAT"),
        "{}",
        failure.error
    );
    assert_eq!(only_event(&processor), before);
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
    let is_cancelled = |processor: &TestProcessor| {
        let event = processor.storage.event(uid).expect("event stored");
        value(&first_vevent(event), "STATUS") == "CANCELLED"
    };

    // In order: request, then cancellation.
    let mut processor = self::processor(true);
    process(&mut processor, "test-11-2.eml");
    let status = process(&mut processor, "test-11-1.eml");
    assert!(status.ends_with(", new sequence: 0"), "{status}");
    assert!(is_cancelled(&processor));

    // Out of order: the cancellation is processed first. It is stored and
    // must not be undone by the older request.
    let mut processor = self::processor(true);
    let status = process(&mut processor, "test-11-1.eml");
    assert!(status.starts_with("Stored new event with UID "), "{status}");
    let status = process(&mut processor, "test-11-2.eml");
    assert!(
        status.starts_with("Not processing older event (sequence: 0 vs 0, DTSTAMP comparison)"),
        "{status}"
    );
    assert!(is_cancelled(&processor));

    // Configured to ignore cancellations for unknown events: the request
    // that follows creates the event, which then is not cancelled.
    let mut processor = self::processor(true);
    processor.config.ignore_unknown_cancellations = true;
    let status = process(&mut processor, "test-11-1.eml");
    assert!(
        status.starts_with("Ignoring cancellation for unknown event with UID "),
        "{status}"
    );
    assert_eq!(processor.storage.event_count(), 0);
    process(&mut processor, "test-11-2.eml");
    assert!(!is_cancelled(&processor));
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
    rejected(
        "test-12.eml",
        "malformed content line 8 without ':': ORGANIZER;CN=Sixt SE",
    );
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
    let stored = |processor: &mut TestProcessor| processor.storage.event(uid).unwrap().clone();

    // 17-0 is itself an occurrence update and arrives first.
    let status = process(&mut processor, "test-17-0.eml");
    assert!(status.starts_with("Stored new event with UID"), "{status}");
    assert_eq!(stored(&mut processor).sequence, 7);

    for (name, expected) in [
        (
            "test-17-1.eml",
            "Updated parent event while preserving instances",
        ),
        // The 16 May occurrence is already stored from 17-0 with sequence 7;
        // 17-2 is an older version of it (sequence 2).
        (
            "test-17-2.eml",
            "Not processing older event (sequence: 2 vs 7",
        ),
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
    let may_16 = occurrences
        .iter()
        .find(|o| value(o, "RECURRENCE-ID") == "20250516T150000")
        .unwrap();
    assert_eq!(value(may_16, "SEQUENCE"), "7", "the newest version is kept");
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
    let event = processor.storage.event(uid).unwrap().clone();
    assert_eq!(event.sequence, 2);
    assert_eq!(occurrences(&event).len(), 1);

    let status = process(&mut processor, "test-18-1.eml");
    assert!(
        status.starts_with("Updated parent event while preserving instances"),
        "{status}"
    );
    let event = processor.storage.event(uid).unwrap().clone();
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
        "test-03-3.eml",
        "test-10.eml",
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
    // issues/archive/bug-missing-schedule-agent-client.md: without SCHEDULE-AGENT=CLIENT
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

/// Puts a mail's event into storage unchanged, as a calendar client or
/// calmailproc 0.1 would have stored it: without any SCHEDULE-AGENT.
fn store_as_is(processor: &mut TestProcessor, raw: &[u8]) {
    let event = calmailproc::email::parse(raw).unwrap().event.unwrap();
    assert!(!event.raw_data.contains("SCHEDULE-AGENT=CLIENT"));
    processor.storage.put(&event);
}

#[test]
fn schedule_agent_of_already_stored_participants_is_not_changed() {
    // If the calendar owner is the organizer, a server must send an attendee
    // a CANCEL when the attendee goes from the default to SCHEDULE-AGENT=CLIENT
    // (RFC 6638, section 3.2.1.2). An event that is already on the server
    // without the parameter must therefore stay without it.
    let mut processor = processor(true);
    store_as_is(&mut processor, &mail("test-05-1.eml"));

    let status = process(&mut processor, "test-05-2.eml");
    assert!(status.starts_with("Updated attendee status"), "{status}");
    let replied = only_event(&processor);
    assert_eq!(
        partstat(&replied, "markus.brechtel@uk-koeln.de").as_deref(),
        Some("ACCEPTED")
    );
    for (participant, agent) in schedule_agents(&replied) {
        assert_eq!(agent, None, "{participant} must not gain SCHEDULE-AGENT");
    }
}

#[test]
fn schedule_agent_follows_the_stored_event_on_updates() {
    let uid = "schedule-agent-update";
    let with_attendees = |sequence: u32, attendees: &str| {
        String::from_utf8(parent_mail(uid, sequence))
            .unwrap()
            .replace(
                "ORGANIZER:organizer@example.com",
                &format!("ORGANIZER:mailto:organizer@example.com\n{attendees}"),
            )
            .into_bytes()
    };

    // Stored by something else: A without the parameter, B with NONE.
    let mut processor = processor(true);
    store_as_is(
        &mut processor,
        &with_attendees(
            0,
            "ATTENDEE:mailto:a@example.com\nATTENDEE;SCHEDULE-AGENT=NONE:mailto:b@example.com",
        ),
    );

    // The update mail says CLIENT for A, nothing for B, and adds C.
    let status = process_raw(
        &mut processor,
        &with_attendees(
            1,
            "ATTENDEE;SCHEDULE-AGENT=CLIENT:MAILTO:A@example.com\nATTENDEE:mailto:b@example.com\nATTENDEE:mailto:c@example.com",
        ),
    );
    assert!(status.ends_with(", new sequence: 1"), "{status}");

    let agents = schedule_agents(&only_event(&processor));
    let agent_of = |address: &str| {
        agents
            .iter()
            .find(|(participant, _)| participant.to_lowercase().ends_with(address))
            .unwrap_or_else(|| panic!("{address} missing in {agents:?}"))
            .1
            .clone()
    };
    assert_eq!(
        agent_of("organizer@example.com"),
        None,
        "stored without: stays without"
    );
    assert_eq!(
        agent_of("a@example.com"),
        None,
        "stored without: stays without"
    );
    assert_eq!(
        agent_of("b@example.com").as_deref(),
        Some("NONE"),
        "stored value is kept"
    );
    assert_eq!(
        agent_of("c@example.com").as_deref(),
        Some("CLIENT"),
        "new attendee"
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

/// A METHOD:REPLY mail for the event `uid` with this ATTENDEE line.
fn synthetic_reply(
    uid: &str,
    attendee: &str,
    comment: Option<&str>,
    recurrence_id: Option<&str>,
) -> Vec<u8> {
    let mut lines = vec![
        format!("UID:{uid}"),
        "DTSTAMP:20250311T090000Z".to_string(),
        attendee.to_string(),
    ];
    lines.extend(comment.map(str::to_string));
    lines.extend(recurrence_id.map(str::to_string));
    format!(
        "From: someone@example.com\nTo: organizer@example.com\nSubject: Reply\nMIME-Version: 1.0\nContent-Type: text/calendar; method=REPLY; charset=UTF-8\n\nBEGIN:VCALENDAR\nPRODID:-//Test//EN\nVERSION:2.0\nMETHOD:REPLY\nBEGIN:VEVENT\n{}\nEND:VEVENT\nEND:VCALENDAR\n",
        lines.join("\n")
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
    store.put(&Event {
        uid: "multi-recurring-event".to_string(),
        raw_data: ical::encode(&calendar).unwrap(),
        summary: "Multi-Instance Meeting".to_string(),
        method: "REQUEST".to_string(),
        sequence: 1,
    });

    let event = store.event("multi-recurring-event").unwrap().clone();
    assert_eq!(parents(&event).len(), 1);
    let occurrences = occurrences(&event);
    assert_eq!(occurrences.len(), 2);
    assert_eq!(value(&occurrences[0], "SUMMARY"), "Modified Instance 1");
    assert_eq!(value(&occurrences[1], "SUMMARY"), "Modified Instance 2");
}

#[test]
fn test_19_uid_with_space() {
    let mut processor = processor(true);
    process(&mut processor, "test-19.eml");
    let event = only_event(&processor);
    assert_eq!(
        event.uid,
        "TLxbpdasn Fdsxaohd-gwoiosg.tlxbpdasn.ccs-znarxou-megtko-0"
    );
    assert_eq!(occurrences(&event).len(), 1);
}

#[test]
fn test_20_calendar_params_on_multipart() {
    let mut processor = processor(true);
    process(&mut processor, "test-20.eml");
    let event = only_event(&processor);
    assert_eq!(
        event.uid,
        "QEdrhcdfk Egxdczey-xfvzncf.qedrhcdfk.ulq-rqadavl-haccih-7"
    );
    assert_eq!(parents(&event).len(), 1);
    assert_eq!(occurrences(&event).len(), 2);
}

#[test]
fn test_21_empty_line_in_folded_attendee() {
    let mut processor = processor(true);
    process(&mut processor, "test-21.eml");
    let event = only_event(&processor);
    assert_eq!(
        event.uid,
        "174605481146O13707L6K9299P89O142968165632307713QV0SAYH58677921159580335815132401EM4K58SBI6393M532C57GJS7P64F3B75"
    );
    assert_eq!(parents(&event).len(), 1);
    assert_eq!(occurrences(&event).len(), 226);
}

#[test]
fn test_22_empty_line_in_folded_description() {
    let mut processor = processor(true);
    process(&mut processor, "test-22.eml");
    let event = only_event(&processor);
    assert_eq!(
        event.uid,
        "24601348613944.BrjUYxCVA4VyKK_GbMht-s6@iopchto.txl-qosayijzh.js"
    );
    assert_eq!(parents(&event).len(), 1);
    assert_eq!(occurrences(&event).len(), 1);
}

#[test]
fn test_23_instance_without_dtstamp() {
    let mut processor = processor(true);
    process(&mut processor, "test-23.eml");
    let event = only_event(&processor);
    assert_eq!(
        event.uid,
        "966789746014E84833N6V8576A38Y8670372498011885OY125G9FN36460763984607957743334999GRUX1I0A9EI98438S4Z56BF4XG41C2HX"
    );
    assert_eq!(parents(&event).len(), 1);
    assert_eq!(occurrences(&event).len(), 1);
    for vevent in calendar(&event).events() {
        assert_eq!(
            vevent.props("DTSTAMP").count(),
            1,
            "every VEVENT needs one DTSTAMP"
        );
    }
}

// --- Mails with several events, options, and repairs ----------------------

/// A calendar mail with any number of VEVENTs, each given as its lines.
fn calendar_mail(method: &str, vevents: &[Vec<String>]) -> Vec<u8> {
    let events: String = vevents
        .iter()
        .map(|lines| format!("BEGIN:VEVENT\n{}\nEND:VEVENT\n", lines.join("\n")))
        .collect();
    format!(
        "From: organizer@example.com\nTo: attendee@example.com\nSubject: Meeting\nDate: Mon, 05 May 2025 10:00:00 +0200\nMIME-Version: 1.0\nContent-Type: text/calendar; method={method}; charset=UTF-8\n\nBEGIN:VCALENDAR\nPRODID:-//Test//EN\nVERSION:2.0\nMETHOD:{method}\n{events}END:VCALENDAR\n"
    )
    .into_bytes()
}

fn parent_lines(uid: &str, sequence: u32) -> Vec<String> {
    vec![
        format!("UID:{uid}"),
        format!("SEQUENCE:{sequence}"),
        "SUMMARY:Series".to_string(),
        "DTSTART:20250505T100000Z".to_string(),
        "RRULE:FREQ=WEEKLY;COUNT=5".to_string(),
        "DTSTAMP:20250501T120000Z".to_string(),
    ]
}

fn occurrence_lines(uid: &str, day: &str, sequence: u32) -> Vec<String> {
    vec![
        format!("UID:{uid}"),
        format!("SEQUENCE:{sequence}"),
        format!("SUMMARY:Moved {day} v{sequence}"),
        format!("DTSTART:{day}T140000Z"),
        format!("RECURRENCE-ID:{day}T100000Z"),
        "DTSTAMP:20250501T120000Z".to_string(),
    ]
}

fn summaries(components: &[Component]) -> Vec<String> {
    components.iter().map(|c| value(c, "SUMMARY")).collect()
}

#[test]
fn all_events_of_a_mail_are_processed_for_a_new_event() {
    let uid = "series-with-exceptions";
    let mut processor = processor(true);
    let mail = calendar_mail(
        "REQUEST",
        &[
            parent_lines(uid, 0),
            occurrence_lines(uid, "20250512", 0),
            occurrence_lines(uid, "20250519", 0),
        ],
    );
    assert_eq!(
        process_raw(&mut processor, &mail),
        format!("Stored new event with UID {uid}")
    );
    let event = only_event(&processor);
    assert_eq!(parents(&event).len(), 1);
    assert_eq!(occurrences(&event).len(), 2);
}

#[test]
fn all_events_of_a_mail_are_merged_into_a_stored_event() {
    let uid = "series-update";
    let mut processor = processor(true);
    process_raw(
        &mut processor,
        &calendar_mail("REQUEST", &[parent_lines(uid, 0)]),
    );
    process_raw(
        &mut processor,
        &calendar_mail("REQUEST", &[occurrence_lines(uid, "20250512", 1)]),
    );

    // One mail: a new parent, a newer version of the stored occurrence, an
    // occurrence that is new, in an order that does not start with the parent.
    let mail = calendar_mail(
        "REQUEST",
        &[
            occurrence_lines(uid, "20250512", 2),
            parent_lines(uid, 3),
            occurrence_lines(uid, "20250526", 2),
        ],
    );
    let status = process_raw(&mut processor, &mail);
    assert_eq!(
        status,
        format!("Updated parent event while preserving instances with UID {uid}")
    );
    let event = only_event(&processor);
    assert_eq!(value(&parents(&event)[0], "SEQUENCE"), "3");
    assert_eq!(
        summaries(&occurrences(&event)),
        ["Moved 20250512 v2", "Moved 20250526 v2"]
    );
    assert_eq!(event.sequence, 3, "the parent is the first event");
}

#[test]
fn several_occurrences_in_one_mail_are_all_merged() {
    let uid = "two-occurrences";
    let mut processor = processor(true);
    process_raw(
        &mut processor,
        &calendar_mail("REQUEST", &[parent_lines(uid, 0)]),
    );
    let status = process_raw(
        &mut processor,
        &calendar_mail(
            "REQUEST",
            &[
                occurrence_lines(uid, "20250512", 1),
                occurrence_lines(uid, "20250519", 1),
            ],
        ),
    );
    assert_eq!(
        status,
        format!("Updated recurring event instance with UID {uid}")
    );
    assert_eq!(occurrences(&only_event(&processor)).len(), 2);
}

#[test]
fn each_component_is_checked_against_its_own_stored_version() {
    let uid = "per-component";
    let mut processor = processor(true);
    process_raw(
        &mut processor,
        &calendar_mail(
            "REQUEST",
            &[parent_lines(uid, 5), occurrence_lines(uid, "20250512", 5)],
        ),
    );

    // An older parent with a newer occurrence: only the occurrence is applied.
    let status = process_raw(
        &mut processor,
        &calendar_mail(
            "REQUEST",
            &[parent_lines(uid, 4), occurrence_lines(uid, "20250512", 6)],
        ),
    );
    assert_eq!(
        status,
        format!("Updated recurring event instance with UID {uid}")
    );
    let event = only_event(&processor);
    assert_eq!(value(&parents(&event)[0], "SEQUENCE"), "5");
    assert_eq!(summaries(&occurrences(&event)), ["Moved 20250512 v6"]);

    // Everything older: nothing changes.
    let status = process_raw(
        &mut processor,
        &calendar_mail(
            "REQUEST",
            &[parent_lines(uid, 1), occurrence_lines(uid, "20250512", 2)],
        ),
    );
    assert_eq!(
        status,
        format!("Not processing older event (sequence: 1 vs 5, DTSTAMP comparison) with UID {uid}")
    );
    assert_eq!(only_event(&processor), event);

    // An older cancellation of the occurrence does not cancel it either.
    process_raw(
        &mut processor,
        &calendar_mail("CANCEL", &[occurrence_lines(uid, "20250512", 3)]),
    );
    assert!(!occurrences(&only_event(&processor))[0].has_prop("STATUS"));
}

#[test]
fn events_with_different_uids_in_one_mail_are_stored_separately() {
    let mut processor = processor(true);
    let mail = calendar_mail(
        "PUBLISH",
        &[
            parent_lines("published-a", 0),
            parent_lines("published-b", 0),
        ],
    );
    assert_eq!(
        process_raw(&mut processor, &mail),
        "Stored new event with UID published-a; Stored new event with UID published-b"
    );
    assert_eq!(processor.storage.event_count(), 2);
    for uid in ["published-a", "published-b"] {
        let event = processor.storage.event(uid).unwrap();
        assert_eq!(calendar(event).events().count(), 1);
        assert_eq!(value(&first_vevent(event), "UID"), uid);
    }
}

#[test]
fn cancelled_event_is_deleted_if_configured() {
    let mut processor = processor(true);
    processor.config.delete_cancelled = true;
    process(&mut processor, "test-01-1.eml");
    assert_eq!(processor.storage.event_count(), 1);
    let status = process(&mut processor, "test-01-2.eml");
    assert!(
        status.starts_with("Deleted cancelled event with UID "),
        "{status}"
    );
    assert_eq!(processor.storage.event_count(), 0);

    // Nothing to delete, and nothing is stored to remember it by.
    let status = process(&mut processor, "test-01-2.eml");
    assert!(
        status.starts_with("Ignoring cancellation for unknown event"),
        "{status}"
    );
    assert_eq!(processor.storage.event_count(), 0);
}

#[test]
fn cancelled_occurrence_is_removed_from_the_series_if_configured() {
    let mut processor = processor(true);
    processor.config.delete_cancelled = true;
    process(&mut processor, "test-03-1.eml");
    process(&mut processor, "test-03-3.eml"); // changes the 20 March occurrence
    let status = process(&mut processor, "test-03-2.eml"); // cancels 27 March
    assert!(
        status.starts_with("Removed cancelled occurrence from event with UID "),
        "{status}"
    );

    let event = only_event(&processor);
    let parent = &parents(&event)[0];
    let exdate = parent.prop("EXDATE").expect("the occurrence is excluded");
    assert_eq!(exdate.value, "20250327T150000");
    assert!(
        exdate.param("TZID").is_some(),
        "same time zone as the series"
    );
    assert_eq!(occurrences(&event).len(), 1, "the changed occurrence stays");
    assert_eq!(
        value(&occurrences(&event)[0], "RECURRENCE-ID"),
        "20250320T150000"
    );

    // Processing the cancellation again does not add a second EXDATE.
    process(&mut processor, "test-03-2.eml");
    assert_eq!(
        parents(&only_event(&processor))[0].props("EXDATE").count(),
        1
    );
}

#[test]
fn untrusted_calendar_data_is_sanitized() {
    let uid = "sanitize";
    let mut lines = parent_lines(uid, 0);
    lines.extend(
        [
            "ATTACH:file:///home/user/.ssh/id_rsa",
            "ATTACH;FMTTYPE=application/pdf:https://example.com/agenda.pdf",
            "ORGANIZER;SCHEDULE-FORCE-SEND=REQUEST:mailto:organizer@example.com",
            "ATTENDEE;SCHEDULE-STATUS=1.2;schedule-force-send=REPLY;CN=A:mailto:a@example.com",
            "BEGIN:VALARM",
            "ACTION:AUDIO",
            "TRIGGER:-PT15M",
            "ATTACH:FILE:///etc/passwd",
            "END:VALARM",
        ]
        .map(String::from),
    );
    let mut processor = processor(true);
    process_raw(&mut processor, &calendar_mail("REQUEST", &[lines]));

    let vevent = first_vevent(&only_event(&processor));
    let attachments: Vec<&str> = vevent.props("ATTACH").map(|a| a.value.as_str()).collect();
    assert_eq!(
        attachments,
        [
            "xfile:///home/user/.ssh/id_rsa",
            "https://example.com/agenda.pdf"
        ]
    );
    assert_eq!(
        vevent.components[0].value("ATTACH"),
        Some("xFILE:///etc/passwd")
    );
    for participant in vevent
        .properties
        .iter()
        .filter(|p| p.name == "ORGANIZER" || p.name == "ATTENDEE")
    {
        let params: Vec<&str> = participant.params.iter().map(|(k, _)| k.as_str()).collect();
        assert!(
            !params
                .iter()
                .any(|k| k.to_uppercase().starts_with("SCHEDULE-")
                    && !k.eq_ignore_ascii_case("SCHEDULE-AGENT")),
            "{params:?}"
        );
        assert_eq!(participant.param("SCHEDULE-AGENT"), Some("CLIENT"));
    }
}

#[test]
fn alarms_are_kept_unless_configured_otherwise() {
    let has_alarm = |processor: &TestProcessor| {
        first_vevent(&only_event(processor))
            .components
            .iter()
            .any(|c| c.name == "VALARM")
    };
    let mut keeping = processor(true);
    process(&mut keeping, "test-10.eml");
    assert!(has_alarm(&keeping));

    let mut dropping = processor(true);
    dropping.config.drop_alarms = true;
    process(&mut dropping, "test-10.eml");
    assert!(!has_alarm(&dropping));
}

#[test]
fn missing_dtstamp_is_taken_from_the_mail_date() {
    let uid = "no-dtstamp-anywhere";
    let lines: Vec<String> = parent_lines(uid, 0)
        .into_iter()
        .filter(|line| !line.starts_with("DTSTAMP"))
        .collect();
    let mail = calendar_mail("REQUEST", &[lines]);
    let mut processor = processor(true);
    process_raw(&mut processor, &mail);
    let event = only_event(&processor);
    // Date: Mon, 05 May 2025 10:00:00 +0200
    assert_eq!(value(&first_vevent(&event), "DTSTAMP"), "20250505T080000Z");

    // The same mail again gives the same event, not a new timestamp.
    process_raw(&mut processor, &mail);
    assert_eq!(only_event(&processor), event);
}

#[test]
fn a_concurrent_change_is_read_again_and_merged() {
    let uid = "concurrent";
    let mut processor = processor(true);
    process_raw(
        &mut processor,
        &calendar_mail("REQUEST", &[parent_lines(uid, 0)]),
    );

    // The first write fails as if someone changed the event in between.
    processor.storage.conflicts_to_inject = 1;
    let status = process_raw(
        &mut processor,
        &calendar_mail("REQUEST", &[parent_lines(uid, 1)]),
    );
    assert_eq!(
        status,
        format!("Updated event with UID {uid}, new sequence: 1")
    );
    assert_eq!(only_event(&processor).sequence, 1);

    // It gives up if that keeps happening, and says why.
    processor.storage.conflicts_to_inject = 10;
    let failure = processor
        .process_email(&calendar_mail("REQUEST", &[parent_lines(uid, 2)]))
        .unwrap_err();
    assert!(
        failure
            .error
            .to_string()
            .contains("changed by someone else"),
        "{}",
        failure.error
    );
    assert_eq!(only_event(&processor).sequence, 1);
}
