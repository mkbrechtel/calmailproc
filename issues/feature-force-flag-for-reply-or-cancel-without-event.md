# Title: Store a reply or cancellation for an unknown event only when forced

**Type:** Feature Request

**Status:** Open

## Description

When a `METHOD:REPLY` or `METHOD:CANCEL` arrives and no event with that UID is
stored, calmailproc stores the mail's calendar data as a new event.

For a reply this is wrong in the normal case. A reply is not an event: it
holds one attendee and a summary such as "Accepted: ...". Stored as an event,
it also blocks the real invitation if that is processed later, because the
reply is newer:

```
test-05-2 (reply), then test-05-1 (invitation)
-> stored: "Akzeptiert: Einladung: Test 5" with one attendee
-> the invitation is skipped as older
```

`tests/order.rs` documents this as a known gap: tests 05 and 06 are the only
non-recurring scenarios whose result depends on the processing order.

For a cancellation, storing it is sometimes useful: test 11 relies on a
cancellation that arrives before its request still winning. But a
cancellation for an event that was never in the calendar is usually noise.

## Expected Behavior

By default:

- A reply for an unknown event is not stored. Status message, for example:
  `Ignoring reply for unknown event with UID ...`.
- A cancellation for an unknown event is not stored either. Status message,
  for example: `Ignoring cancellation for unknown event with UID ...`.

With a force option, the current behaviour: the data is stored as a new
event. This is for deliberate use, for example from a mail client such as
mutt, when the user wants this particular mail in the calendar.

## Proposed Solution

- A flag `-force` (and `processor.force` in the config file) that the
  processor checks in `process_event_reply()` and, for `METHOD:CANCEL`, in
  `process_event()` before storing a new event.
- Decide whether one flag covers both cases or whether they are separate
  (`-force-reply`, `-force-cancel`).
- Test 11 needs a decision: either it runs with the force option, or
  cancel-before-request is kept as the default for cancellations and only
  replies need forcing. The second keeps the out-of-order protection that
  test 11 was written for.

## Additional Context

Thunderbird and Evolution both ignore a reply or cancellation when the event
is not in the calendar (`calItipUtils.sys.mjs`: "has already been processed",
"item has been previously removed from the calendar"; `e-cal-meta-backend.c`:
`OBJECT_NOT_FOUND` for a cancel).
