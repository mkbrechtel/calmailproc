# Title: Write events with If-Match / If-None-Match, as Evolution does

**Type:** Improvement

**Status:** Resolved

## Description

calmailproc reads the stored event, merges the mail into it and writes the
result with an unconditional `PUT`. If anything else changes the event between
the read and the write, that change is lost: a second calmailproc processing
another mail for the same UID (two deliveries at once), or a calendar client.

The window is short and for a single user the risk is small, so this is an
improvement, not an urgent bug.

evolution-data-server sends the ETag it got when reading
(`ecb_caldav_save_component_sync` in `e-cal-backend-caldav.c`): `If-Match`
with the ETag when overwriting, and no overwrite of an existing resource when
creating.

## Expected Behavior

- Updating an event: `PUT` with `If-Match: <etag from the GET>`.
- Creating an event: `PUT` with `If-None-Match: *`.
- On `412 Precondition Failed`: read the event again, merge again, write
  again; give up with an error after a few attempts.

## Actual Behavior

Unconditional `PUT`; the last writer wins.

## Proposed Solution

- `CalDavStorage::get_event()` keeps the `ETag` response header.
- `store_event()` sends the matching condition.
- The retry belongs in the processor, because merging again needs the
  processor's logic: on a conflict error from storage, run the processing of
  that mail once more.

This shares the "location and version of the stored event" value with
`bug-event-lookup-assumes-uid-filename.md` and is best done together with it.

The fake server in `tests/end_to_end.rs` needs ETags and the two conditions.

## Resolution

**Resolved in:** version 0.2.0 (branch `rewrite-in-rust`)
**Resolved by:** mkbrechtel, with Claude

`CalDavStorage::store_event()` sends `If-Match` with the ETag that was read
when it replaces an event and `If-None-Match: *` when it creates one;
`delete_event()` sends `If-Match` too. If the server gave no ETag, the write
is unconditional as before.

A failed condition (HTTP 412) becomes `StoreError::Conflict`. The processor
(`process_calendar()` in `src/processor/mod.rs`) then reads the event again,
merges again and writes again, up to three times, and fails with "the stored
event was changed by someone else 3 times in a row" after that.

Xandikos also answers 412 when it rejects calendar data, with a WebDAV error
body naming a CalDAV precondition such as `valid-calendar-data`. That is
reported as an error with the server's explanation, not retried
(`Reply::is_caldav_error()`).

`MemoryStorage` checks versions the same way, and can inject conflicts for
tests.

Tests: `a_concurrent_change_is_read_again_and_merged` in
`tests/processor.rs`, `writes_against_an_outdated_version_conflict` in
`src/storage/memory.rs`, and the conditions in the fake server of
`tests/end_to_end.rs`.
