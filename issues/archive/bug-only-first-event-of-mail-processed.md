# Title: Only part of a calendar with several VEVENTs is processed

**Type:** Bug

**Status:** Resolved

## Description

A calendar attachment can hold more than one `VEVENT`. The usual case is a
recurring event sent together with its exceptions: one parent `VEVENT` and
one `VEVENT` with `RECURRENCE-ID` per changed occurrence, all with the same
UID. Outlook and Exchange send such mails for a series that already has
changed occurrences.

calmailproc decides everything from the first `VEVENT` (UID, sequence,
summary) and does not handle the rest consistently:

1. New event: the whole calendar is stored as it is. This case is right.
2. Update of a stored event without occurrences, or with them
   (`handle_parent_event_update()`): only the first parent `VEVENT` of the
   mail is kept. Exceptions that came in the same mail are dropped.
3. Update of one occurrence (`handle_recurring_event()`): only the first
   `VEVENT` with `RECURRENCE-ID` is merged. A mail with two changed
   occurrences loses the second.
4. `is_recurring_update()` is true as soon as any `VEVENT` has a
   `RECURRENCE-ID`. A mail with a parent and exceptions is therefore treated
   as an update to one occurrence, and its parent is not applied at all.
5. A calendar with several different UIDs (allowed for `METHOD:PUBLISH`):
   everything is stored under the first UID.

Cases 2 to 4 were read from the code and are not covered by a test mail yet.

## Expected Behavior

Every `VEVENT` in the calendar is processed:

- the parent, compared with the stored parent,
- each occurrence, merged into the stored event,
- for different UIDs, each UID as its own event.

## Actual Behavior

See the list above.

## Proposed Solution

Restructure `process_event()` in `src/processor/mod.rs` to work per
component instead of per mail:

1. Group the mail's `VEVENT`s by UID.
2. For each UID, load the stored event once.
3. Apply the parent `VEVENT`, if the mail has one, with the version check.
4. Apply each `VEVENT` with `RECURRENCE-ID` as an occurrence update.
5. Validate and store once per UID.

This is the same restructuring that
`bug-recurring-updates-depend-on-order.md` needs, and both should be done
together.

Tests need mails with: a parent plus two exceptions as a new event; the same
as an update to a stored event; two changed occurrences without parent; and
a `PUBLISH` with two UIDs.

## Additional Context

- Thunderbird processes every item of the message, with exceptions attached
  to their parent (`CalItipItem.sys.mjs` builds the item list;
  `processFoundItems()` loops over it and `updateItem()` installs all
  exceptions). It looks up only the first item's UID, with the comment that
  the iTIP specification requires all items of a message to have the same
  UID.
- evolution-data-server loops over all components of the calendar and
  processes each on its own (`ecmb_receive_objects_sync()`).

## Resolution

**Resolved in:** version 0.2.0 (branch `rewrite-in-rust`)
**Resolved by:** mkbrechtel, with Claude

The processor now works per component (`src/processor/mod.rs`):

1. `split_by_uid()` splits the mail's calendar into one calendar per UID.
   Each is processed as its own event; the status messages are joined with
   `; `. All are processed even if one fails, and the first failure is
   reported.
2. `apply_update()` goes through every `VEVENT` of that UID: the parent
   replaces the stored parent, each `VEVENT` with `RECURRENCE-ID` replaces
   or adds that occurrence. The order of the components in the mail does not
   matter, and a mail with a parent and exceptions is no longer mistaken for
   an update to one occurrence.
3. Time zones (`VTIMEZONE`) that the merged event does not have yet are
   added from the mail.

The event is read, validated and stored once per UID.

Tests in `tests/processor.rs`:
`all_events_of_a_mail_are_processed_for_a_new_event`,
`all_events_of_a_mail_are_merged_into_a_stored_event`,
`several_occurrences_in_one_mail_are_all_merged`,
`events_with_different_uids_in_one_mail_are_stored_separately`, and tests
20, 21 and 22 from the archive import (a series with 2, 226 and 1 changed
occurrences in one mail).
