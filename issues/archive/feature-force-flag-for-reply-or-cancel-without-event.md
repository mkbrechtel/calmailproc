# Title: Store a reply or cancellation for an unknown event only when forced

**Type:** Feature Request

**Status:** Resolved

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

## Resolution

**Resolved in:** version 0.2.0 (branch `rewrite-in-rust`)
**Resolved by:** mkbrechtel, with Claude

Implemented differently for the two cases, because of test 11.

**Replies:** a reply for an event that is not stored is ignored
(`Ignoring reply for unknown event with UID ...`). The flag
`-process-replies-without-event` (`processor.process_replies_without_event`
in the config file) stores it as an event, as before. The flag was first
called `-force` and renamed, since it only concerns replies.

**Cancellations:** a cancellation for an event that is not stored is still
stored by default, as a cancelled event. `test/test-11.md` describes exactly
this case ("appears mostly in maildir mode, when the cancelation is parsed
before the request") and requires that the event stays cancelled whatever
the processing order. Ignoring such cancellations by default would break that
in every maildir run.

Ignoring them is available as an option: `-ignore-unknown-cancellations`
(`processor.ignore_unknown_cancellations`). With it, a request that is
processed after its cancellation creates the event uncancelled.

Tests in `tests/processor.rs`:
`reply_for_unknown_event_is_ignored_unless_configured` and
`test_11_out_of_sequence_mail` (default and with the option).

Still order-dependent: a reply that is processed before its invitation is
lost, since nothing is stored to remember it by. Thunderbird and Evolution
behave the same.
