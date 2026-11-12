//! Tests for the two input modes, stdin and maildir, with in-memory storage.

use std::path::Path;

use calmailproc::processor::{Processor, maildir, stdin};
use calmailproc::storage::MemoryStorage;

fn processor() -> Processor<MemoryStorage> {
    Processor::new(MemoryStorage::new(), true)
}

#[test]
fn stdin_processes_one_email() {
    let mut processor = processor();
    let raw = std::fs::read("test/maildir/cur/test-01-1.eml").unwrap();
    stdin::process_reader(raw.as_slice(), &mut processor).unwrap();
    assert_eq!(processor.storage.event_count(), 1);
}

#[test]
fn stdin_reports_invalid_calendar_data() {
    let mut processor = processor();
    let raw = std::fs::read("test/maildir/cur/test-13.eml").unwrap();
    let error = stdin::process_reader(raw.as_slice(), &mut processor).unwrap_err();
    assert!(
        error.to_string().contains("validation error for event"),
        "{error}"
    );
    assert_eq!(processor.storage.event_count(), 0);
}

#[test]
fn maildir_rejects_missing_path() {
    let error = maildir::process(
        Path::new("/path/that/does/not/exist"),
        &mut processor(),
        false,
    )
    .unwrap_err();
    assert!(
        error.to_string().contains("maildir path does not exist"),
        "{error}"
    );
}

#[test]
fn maildir_processes_all_mails_and_survives_broken_ones() {
    // The test maildir contains mails with invalid calendar data (tests 12-15).
    // They are reported as warnings and must not stop the run.
    for verbose in [false, true] {
        let mut processor = processor();
        maildir::process(Path::new("test/maildir"), &mut processor, verbose).unwrap();
        let events = processor.storage.list_events();
        assert_eq!(
            events.len(),
            19,
            "one event per test with valid calendar data"
        );
        assert!(events.iter().all(|event| !event.uid.is_empty()));
    }
}

#[test]
fn maildir_processes_subfolders_recursively() {
    let root = std::env::temp_dir().join(format!("calmailproc-maildir-{}", std::process::id()));
    let nested = root.join("Calendar").join("Work").join("new");
    std::fs::create_dir_all(&nested).unwrap();
    std::fs::create_dir_all(root.join("tmp")).unwrap();
    std::fs::copy("test/maildir/cur/test-08.eml", nested.join("mail-1")).unwrap();
    std::fs::copy(
        "test/maildir/cur/test-09.eml",
        root.join("tmp").join("ignored"),
    )
    .unwrap();
    std::fs::write(nested.join("maildirfolder"), "").unwrap();

    let mut processor = processor();
    let result = maildir::process(&root, &mut processor, false);
    std::fs::remove_dir_all(&root).unwrap();
    result.unwrap();

    let events = processor.storage.list_events();
    assert_eq!(
        events.len(),
        1,
        "only the mail in Calendar/Work/new is processed"
    );
    assert_eq!(events[0].summary, "Test 8");
}
