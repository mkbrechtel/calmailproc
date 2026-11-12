//! Mails do not always arrive, or get processed, in the order they were sent,
//! and a maildir run may see the same mail again. The stored result must not
//! depend on either.
//!
//! It does not hold for a reply that is processed before its invitation: the
//! reply is ignored (or, with `force`, stored in place of the invitation).
//! It also does not hold with `ignore_unknown_cancellations` or
//! `delete_cancelled`, which keep nothing to remember a cancellation by.

use calmailproc::ical;
use calmailproc::processor::Processor;
use calmailproc::storage::MemoryStorage;

/// Processes the mails in this order and describes what is stored afterwards.
fn stored_after(mails: &[&str]) -> Vec<String> {
    let mut processor = Processor::new(MemoryStorage::new(), true);
    for mail in mails {
        let raw = std::fs::read(format!("test/maildir/cur/{mail}")).unwrap();
        processor
            .process_email(&raw)
            .unwrap_or_else(|failure| panic!("{mail}: {}", failure.error));
    }
    let mut described = Vec::new();
    for event in processor.storage.list_events() {
        let calendar = ical::decode(&event.raw_data).unwrap();
        for vevent in calendar.events() {
            // Everything in the event counts, in a stable order.
            let mut properties: Vec<String> = vevent
                .properties
                .iter()
                .map(|p| {
                    let mut params = p.params.clone();
                    params.sort();
                    format!("{} {:?} {}", p.name, params, p.value)
                })
                .collect();
            properties.sort();
            described.push(properties.join("\n"));
        }
    }
    described.sort();
    described
}

fn permutations<'a>(items: &[&'a str]) -> Vec<Vec<&'a str>> {
    if items.len() <= 1 {
        return vec![items.to_vec()];
    }
    let mut all = Vec::new();
    for index in 0..items.len() {
        let mut rest = items.to_vec();
        let first = rest.remove(index);
        for mut permutation in permutations(&rest) {
            permutation.insert(0, first);
            all.push(permutation);
        }
    }
    all
}

const ORDER_INDEPENDENT: [&[&str]; 8] = [
    &["test-01-1.eml", "test-01-2.eml"],
    &["test-02-1.eml", "test-02-2.eml"],
    &["test-03-1.eml", "test-03-2.eml", "test-03-3.eml"],
    &["test-04-1.eml", "test-04-2.eml"],
    &[
        "test-07-1.eml",
        "test-07-2.eml",
        "test-07-3.eml",
        "test-07-4.eml",
        "test-07-5.eml",
    ],
    &["test-11-1.eml", "test-11-2.eml"],
    &["test-16-1.eml", "test-16-2.eml"],
    &["test-18-0.eml", "test-18-1.eml"],
];

#[test]
fn result_does_not_depend_on_processing_order() {
    for mails in ORDER_INDEPENDENT {
        let expected = stored_after(mails);
        assert!(!expected.is_empty());
        for order in permutations(mails) {
            assert_eq!(stored_after(&order), expected, "order {order:?}");
        }
    }
}

const TEST_17: [&str; 8] = [
    "test-17-0.eml",
    "test-17-1.eml",
    "test-17-2.eml",
    "test-17-3.eml",
    "test-17-4.eml",
    "test-17-5.eml",
    "test-17-6.eml",
    "test-17-7.eml",
];

#[test]
fn recurring_series_does_not_depend_on_processing_order() {
    // Test 17: a parent, updates to three occurrences and two cancellations,
    // with two versions each of the parent and of two occurrences.
    let expected = stored_after(&TEST_17);
    // The parent and every occurrence end at their highest sequence.
    for expected_line in [
        "SEQUENCE [] 5",
        "SEQUENCE [] 6",
        "SEQUENCE [] 7",
        "SEQUENCE [] 3",
        "SEQUENCE [] 4",
    ] {
        assert_eq!(
            expected
                .iter()
                .filter(|event| event.contains(expected_line))
                .count(),
            1,
            "{expected_line}"
        );
    }
    assert_eq!(expected.len(), 5);

    // All 40320 orders take a while; every 97th is a spread of 416 of them.
    for order in permutations(&TEST_17).into_iter().step_by(97) {
        assert_eq!(stored_after(&order), expected, "order {order:?}");
    }
}

#[test]
fn processing_the_same_mails_again_changes_nothing() {
    let all: Vec<&[&str]> = ORDER_INDEPENDENT
        .into_iter()
        .chain([
            &["test-05-1.eml", "test-05-2.eml"] as &[&str],
            &["test-06-1.eml", "test-06-2.eml"],
            &TEST_17,
        ])
        .collect();
    for mails in all {
        let twice: Vec<&str> = mails.iter().chain(mails.iter()).copied().collect();
        assert_eq!(stored_after(&twice), stored_after(mails), "{mails:?}");
    }
}
