# Title: Stored events trigger server-side scheduling mails (missing SCHEDULE-AGENT=CLIENT)

**Type:** Bug

**Status:** Resolved

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

## Resolution

**Resolved in:** version 0.2.0 (branch `rewrite-in-rust`)
**Resolved by:** mkbrechtel, with Claude

`prepare_event_for_storage()` in `src/processor/mod.rs` calls
`set_schedule_agents()` for every event before it is stored:

- A participant (`ORGANIZER` or `ATTENDEE`) that is new on the server gets
  `SCHEDULE-AGENT=CLIENT`, replacing any value the mail carried. That is every
  participant of a new event, and participants added to a stored one.
- A participant that is already in the stored event keeps the stored value,
  including "no parameter".

The second rule differs from the proposal above, which said to set the
parameter on all paths. That would be harmful: RFC 6638, section 3.2.1.2,
requires a server to send an attendee a CANCEL when the organizer's stored
event changes that attendee from the default (`SERVER`) to `CLIENT`. An event
that is already on the server without the parameter, for example one created
in a calendar client or stored by calmailproc 0.1, must stay as it is.
Evolution (evolution-data-server, `ecb_caldav_save_component_sync`) has the
same rule and only sets the parameter when it creates a resource.

Consequence: for events that were stored without the parameter, the server
keeps doing its own scheduling, as it did under 0.1.

Tests in `tests/processor.rs`:
`stored_events_tell_the_server_not_to_schedule`,
`incoming_schedule_agent_server_is_overwritten`,
`schedule_agent_of_already_stored_participants_is_not_changed`,
`schedule_agent_follows_the_stored_event_on_updates`, and a check on every
event stored by the binary in `tests/end_to_end.rs`.

Not verified against a server that actually sends scheduling mails: Xandikos,
used by `test.sh`, has no mail delivery. Whether a given server honours the
parameter has to be checked on that server, with a test event whose
participants are addresses you control.
