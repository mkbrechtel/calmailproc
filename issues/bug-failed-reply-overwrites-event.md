# Title: A reply from an unknown attendee replaces the stored event

**Type:** Bug

**Status:** Open

## Description

When a `METHOD:REPLY` arrives for an event that is in storage, calmailproc
sets the replying attendee's `PARTSTAT` in the stored event. If the replying
address is not among the stored attendees, the update fails and calmailproc
falls back to storing the reply's own calendar data under the event's UID.
This replaces the stored invitation with the reply, which holds only the
replying attendee and a summary such as "Accepted: ...".

A reply must only ever change the one attendee it comes from.

This fallback was carried over unchanged from version 0.1. The most common
trigger in 0.1 (`MAILTO:` versus `mailto:`) is fixed in 0.2 by comparing
addresses case-insensitively, but the fallback itself is still there.

## Steps to Reproduce

1. Process an invitation with attendees A and B.
2. Process a reply for the same UID from an address C that is not an
   attendee (a delegate, an alias of A, or someone the invitation was
   forwarded to).

## Expected Behavior

The stored event is unchanged except that C is added as an attendee with the
`PARTSTAT` from the reply. Status message, for example:
`Added attendee C to event with UID ...`.

## Actual Behavior

The stored invitation is replaced by the reply. The status message is
`Stored reply event with UID ... (attendee update failed)`.

## Proposed Solution

In `update_attendee_status()` in `src/processor/mod.rs`: if no stored
`ATTENDEE` matches, add the reply's `ATTENDEE` property (with its `PARTSTAT`
and `CN`) to the VEVENT the reply refers to (the parent, or the occurrence
named by `RECURRENCE-ID`). New attendees get `SCHEDULE-AGENT=CLIENT` from
`set_schedule_agents()` like any participant that is new on the server.

Remove the fallback in `process_event_reply()` that stores the reply over the
stored event. If the reply cannot be applied at all (no `ATTENDEE`, no
`PARTSTAT`, no matching occurrence), report that and leave the stored event
alone.

## Additional Context

How the others handle it:

- Thunderbird accepts the reply and adds the attendee
  (`calItipUtils.sys.mjs`: "We accepts REPLYs also from previously uninvited
  attendees").
- Evolution asks the user whether to add the unknown attendee
  (`itip-view.c`, `add-unknown-attendee`), and handles delegation
  (`DELEGATED-FROM` / `DELEGATED-TO`) separately.

Neither ever replaces the event with the reply.

What happens to a reply when there is no stored event at all is covered in
`feature-force-flag-for-reply-or-cancel-without-event.md`.
