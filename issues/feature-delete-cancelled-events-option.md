# Title: Option to delete cancelled events

**Type:** Feature Request

**Status:** Open

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
