# Title: Stored events trigger server-side scheduling mails (missing SCHEDULE-AGENT=CLIENT)

**Type:** Bug

**Status:** Open

## Description

calmailproc stores events it received by mail on a CalDAV server. The events
contain `ORGANIZER` and `ATTENDEE` properties, so a CalDAV server with
scheduling support (RFC 6638) treats the `PUT` as a scheduling operation and
performs implicit scheduling: it sends its own iTIP messages (invitations,
replies, cancellations) to the organizer or attendees.

Those people have already been in the mail exchange that calmailproc is
processing, so the server's messages are duplicates. The scheduling has
already happened by mail; the server must not do it again.

RFC 6638 section 7.1 defines the `SCHEDULE-AGENT` property parameter for this.
Its default value is `SERVER`. Setting `SCHEDULE-AGENT=CLIENT` on an
`ORGANIZER` or `ATTENDEE` property tells the server that the client handles
scheduling for that calendar user, and the server must not send scheduling
messages to them.

calmailproc currently stores these properties unchanged, so the default
`SERVER` applies.

## Steps to Reproduce

1. Configure calmailproc against a CalDAV server that has scheduling
   (RFC 6638) and mail delivery enabled.
2. Pipe an invitation mail (`METHOD:REQUEST`) with an `ORGANIZER` and several
   `ATTENDEE` properties into calmailproc.
3. Check the mailboxes of the organizer and the attendees.

## Expected Behavior

The event is stored in the calendar and no further mail is sent by the
CalDAV server. Every `ORGANIZER` and `ATTENDEE` property in the stored event
carries `SCHEDULE-AGENT=CLIENT`, for example:

```
ORGANIZER;CN=Alice;SCHEDULE-AGENT=CLIENT:mailto:alice@example.com
ATTENDEE;CN=Bob;PARTSTAT=NEEDS-ACTION;SCHEDULE-AGENT=CLIENT:mailto:bob@example.com
```

## Actual Behavior

The event is stored without `SCHEDULE-AGENT`, and the server may send another
scheduling mail for the event that was just processed from a mail.

## Proposed Solution

Set the parameter in the one place every stored event passes through:
`prepare_event_for_storage()` in `src/processor/mod.rs`, which already removes
the `METHOD` property before storage.

- For every `VEVENT` in the calendar, including recurrence instances
  (components with `RECURRENCE-ID`), set `SCHEDULE-AGENT=CLIENT` on the
  `ORGANIZER` property and on every `ATTENDEE` property.
- Overwrite an existing `SCHEDULE-AGENT` value, so that an incoming
  `SCHEDULE-AGENT=SERVER` does not survive.
- Apply it on all paths: new events, updates, cancellations, recurring
  instance merges, parent updates and attendee status updates from
  `METHOD:REPLY`. All of them already go through `prepare_event_for_storage()`.

Tests to add:

- A stored event has `SCHEDULE-AGENT=CLIENT` on `ORGANIZER` and on all
  `ATTENDEE` properties.
- After a recurring instance merge, both the parent and the instance
  components carry the parameter.
- After a `METHOD:REPLY` update, the parameter is still present.
- Other parameters (`CN`, `PARTSTAT`, `ROLE`, `RSVP`) are preserved.

## Additional Context

- RFC 6638, section 7.1: "Schedule Agent Parameter". Values are `SERVER`
  (default), `CLIENT` and `NONE`.
- The check belongs in `tests/processor.rs`, on the stored iCalendar data.
- The end-to-end test in `test.sh` runs a local Xandikos with no mail
  delivery, so it cannot observe the duplicate mail itself. The check has to
  be made on the stored iCalendar data.
