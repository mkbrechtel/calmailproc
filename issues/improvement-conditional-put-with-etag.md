# Title: Write events with If-Match / If-None-Match, as Evolution does

**Type:** Improvement

**Status:** Open

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
