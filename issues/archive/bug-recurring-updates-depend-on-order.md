# Title: Updates within a recurring series depend on the processing order

**Type:** Bug

**Status:** Resolved

## Description

For ordinary events the stored result does not depend on the order in which
mails are processed: an older version never replaces a newer one. This is
tested in `tests/order.rs` for every order of the mails of tests 01, 02, 03,
04, 07, 11, 16 and 18.

Within a recurring series this does not hold. Two version checks are missing:

1. An update to one occurrence replaces the stored occurrence with the same
   `RECURRENCE-ID` without comparing sequence or DTSTAMP. The mail processed
   last wins.
2. Once the stored event contains any occurrence update, an update to the
   parent event replaces the stored parent without comparing either
   (`process_event()`, the `existing.is_recurring_update()` branch).

## Steps to Reproduce

Test 17 has eight mails for one series. Processing them in all 40320 orders
gives eight different end states, from three independent "last one wins"
choices:

| Component | Mails | Sequence that should win | Sequence stored, depending on order |
|---|---|---|---|
| Parent | 17-1, 17-7 | 5 | 0 or 5 |
| Occurrence 9 May | 17-3, 17-6 | 6 | 1 or 6 |
| Occurrence 16 May | 17-2, 17-0 | 7 | 2 or 7 |

Even the plain file order (17-0 to 17-7) is affected: the 16 May occurrence
ends at sequence 2, although sequence 7 was received first.

## Expected Behavior

One end state for every order: parent at sequence 5, 9 May at 6, 16 May
at 7, and the two cancelled occurrences cancelled.

## Actual Behavior

Eight different end states; see the table.

## Proposed Solution

Compare per component, not per stored event:

- In `handle_recurring_event()`: if an occurrence with the same
  `RECURRENCE-ID` is stored, replace it only if the new one is not older
  (`ical::compare_events` logic on the two components: SEQUENCE, then
  DTSTAMP). A cancellation is subject to the same check.
- In `process_event()`: compare a parent update with the stored parent
  component, also when occurrences are stored. If the stored event has no
  parent yet (only occurrences), the parent is always applied.
- `Event.sequence` is the sequence of the first `VEVENT`, which is not
  always the parent. Comparing needs the sequence and DTSTAMP of the
  specific component.

Then add test 17 to the order-independent scenarios in `tests/order.rs`.
With eight mails that is 40320 orders, which takes a few seconds in a release
build; a fixed sample of orders is enough for the debug test run.

## Additional Context

Processing the same mails twice in the same order already gives the same
result for all scenarios, including test 17
(`processing_the_same_mails_again_changes_nothing`).

Related: `bug-only-first-event-of-mail-processed.md` needs the same
per-component restructuring of the processor.

## Resolution

**Resolved in:** version 0.2.0 (branch `rewrite-in-rust`)
**Resolved by:** mkbrechtel, with Claude

`apply_update()` in `src/processor/mod.rs` compares every component of a mail
with the stored component it would replace (`ical::compare_components()`:
SEQUENCE, then DTSTAMP): the parent with the stored parent, an occurrence
with the stored occurrence of the same `RECURRENCE-ID`. The stored one stays
if it is newer. A parent update is compared also when occurrences are stored,
and a cancellation is subject to the same check. A cancelled occurrence keeps
the sequence and DTSTAMP of its cancellation, so later mails are compared
against that.

Test 17 now ends in one state for every processing order: parent at
sequence 5, 9 May at 6, 16 May at 7, and the two cancelled occurrences
cancelled. `recurring_series_does_not_depend_on_processing_order` in
`tests/order.rs` checks 416 of the 40320 orders in the normal test run, and
all 40320 with `CALMAILPROC_ALL_ORDERS=1 cargo test --release --test order`.

In the plain file order, test-17-2 is now reported as
`Not processing older event (sequence: 2 vs 7, ...)`, because the 16 May
occurrence is already stored from test-17-0 with sequence 7.

Also tested: `each_component_is_checked_against_its_own_stored_version` in
`tests/processor.rs`.
