# Title: A reply whose attendee is not in the stored event replaces that event

**Type:** Bug

**Status:** Open

## Description

When a `METHOD:REPLY` arrives for an event that is in storage, calmailproc
sets the replying attendee's `PARTSTAT` in the stored event. If that fails,
for example because the replying address is not among the stored attendees,
the reply's own calendar data is stored under the event's UID. This replaces
the stored invitation with the reply, which holds only the replying attendee
and a summary such as "Accepted: ...".

This behaviour was carried over unchanged from version 0.1. The most common
trigger in 0.1 (`MAILTO:` versus `mailto:`) is fixed in 0.2 by comparing
addresses case-insensitively, but the fallback itself is still there.

## Steps to Reproduce

1. Process an invitation with attendees A and B.
2. Process a reply for the same UID from an address C that is not an
   attendee (a delegate, or an alias of A).

## Expected Behavior

The stored invitation is kept. The reply is reported as not applicable.

## Actual Behavior

The stored invitation is replaced by the reply. The status message is
`Stored reply event with UID ... (attendee update failed)`.

## Proposed Solution

In `process_event_reply()` in `src/processor/mod.rs`, do not store the reply
when `update_attendee_status()` fails and a stored event exists. Return a
status such as `Ignoring reply from unknown attendee ... for event with UID ...`.

Decide separately whether a reply without any stored event should still be
stored (`Stored new reply event with UID ...`), or ignored as well.
