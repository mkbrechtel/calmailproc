# Title: Option to delete cancelled events

**Type:** Feature Request

**Status:** Resolved

## Description

When a cancellation is processed, calmailproc keeps the event and marks it
with `STATUS:CANCELLED` (for a single occurrence of a recurring event: that
occurrence). This is deliberate and stays the default: the calendar shows what
was cancelled, and a cancellation that arrives before its request still wins.

Some users will prefer cancelled events to disappear, which is what
Thunderbird and Evolution do: they delete the event, or remove the occurrence
from the series.

## Expected Behavior

A config option, off by default, for example:

```yaml
processor:
  delete_cancelled: true
```

With it:

- a cancellation of a whole event deletes the stored event,
- a cancellation of one occurrence removes that occurrence: an `EXDATE` on
  the parent event, and the stored exception for that `RECURRENCE-ID` is
  dropped.

## Proposed Solution

- Add `delete_event()` to the `Storage` trait (`DELETE` on the resource for
  CalDAV; `MemoryStorage` already has it).
- Handle the option in `process_event()` and `handle_recurring_event()` in
  `src/processor/mod.rs`.

## Additional Context

With deletion there is nothing left to remember the cancellation by, so a
request that arrives after its cancellation would create the event again
(the case test 11 covers). The documentation of the option should say so.

## Resolution

**Resolved in:** version 0.2.0 (branch `rewrite-in-rust`)
**Resolved by:** mkbrechtel, with Claude

`processor.delete_cancelled` in the config file, or `-delete-cancelled`. Off
by default.

With it (`apply_update()` in `src/processor/mod.rs`):

- A cancellation of the whole event deletes the stored event
  (`Deleted cancelled event with UID ...`). `Storage` has `delete_event()`
  for this.
- A cancellation of one occurrence removes the stored exception for that
  `RECURRENCE-ID`, if there is one, and adds an `EXDATE` with the same value
  and time zone to the parent event
  (`Removed cancelled occurrence from event with UID ...`).
- A cancellation that is older than the stored version is skipped, as for
  any update.
- A cancellation for an event that is not stored is ignored.

As the description says, nothing is left to remember the cancellation by: a
request processed afterwards creates the event or occurrence again.

Tests in `tests/processor.rs`: `cancelled_event_is_deleted_if_configured`,
`cancelled_occurrence_is_removed_from_the_series_if_configured`.
