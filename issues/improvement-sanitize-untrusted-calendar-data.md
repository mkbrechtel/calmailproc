# Title: Sanitize calendar data from mails before storing it

**Type:** Improvement

**Status:** Open

## Description

calmailproc stores the calendar data of a mail as it was received, apart from
removing `METHOD` and setting `SCHEDULE-AGENT`. The data comes from whoever
sent the mail and ends up in a calendar that the user's calendar clients
trust.

Evolution sanitizes such data before storing it. Its function
`e_cal_util_sanitize_untrusted()` (evolution-data-server,
`src/calendar/libecal/e-cal-util.c`, since 3.64) does one thing:

- For every `ATTACH` property of the component whose value is a URL
  starting with `file://`, it rewrites the URL to `xfile://...`.

The reason given in the code: an `xfile://` URI "is not inlined". A calendar
client that later sends the event on (a reply, a forward, an update to
attendees) inlines local attachments. An invitation with
`ATTACH:file:///home/user/.ssh/id_rsa` would make the client read that file
and send it. With the scheme changed, the reference is kept but no client
resolves it.

The CalDAV backend calls it through `e_cal_backend_sanitize_untrusted()`,
which leaves alone the URIs that point into the backend's own attachment
cache directory, since those were put there by the backend itself.

It is called for every component received by mail
(`ecmb_receive_object_sync()` in `e-cal-meta-backend.c`), before the
component is created or merged.

That is all it does. It does not touch alarms, other URLs, or descriptions.

## Expected Behavior

Calendar data from a mail cannot make a calendar client act on local files.

## Actual Behavior

`ATTACH` properties are stored unchanged.

## Proposed Solution

In `prepare_event_for_storage()` in `src/processor/mod.rs`, for every
`ATTACH` property in every component (events and their alarms: `VALARM` can
carry `ATTACH` too, which Evolution's function does not cover), if the value
starts with `file:` (any case), either:

- rewrite the scheme as Evolution does (`xfile:`), which keeps the
  information that there was an attachment, or
- remove the property.

calmailproc has no attachment cache of its own, so no exception is needed.

Thunderbird sanitizes differently. For every item of a received message
(`cleanItem()` in `calendar/base/src/CalItipItem.sys.mjs`) it:

- removes all alarms (`item.clearAlarms()`); when the user accepts, the
  user's own default alarms are set instead (`cal.alarms.setDefaultValues`),
- removes all `X-MOZ-` properties, which carry Thunderbird's own state,
- removes `SCHEDULE-AGENT`, `SCHEDULE-FORCE-SEND` and `SCHEDULE-STATUS` from
  the organizer and all attendees, with the comment that RFC 6638 does not
  allow them in scheduling messages sent by mail.

Worth adopting from that:

- `VALARM` components from invitations. An alarm with `ACTION:EMAIL` or
  `ACTION:PROCEDURE`, or `ACTION:AUDIO` with a URL, lets the sender make the
  user's client send mail, run something or fetch a URL. An option to drop
  alarms from mails, or to keep only `ACTION:DISPLAY`, would close this.
  Dropping them by default would be a visible change: test 10 asserts that
  the `VALARM` of an invitation is kept.
- `SCHEDULE-FORCE-SEND` and `SCHEDULE-STATUS` from the mail. calmailproc
  already replaces an incoming `SCHEDULE-AGENT` for participants that are
  new on the server, but passes these two through. `SCHEDULE-FORCE-SEND`
  in particular asks the server to send a scheduling message.

## Additional Context

Whether a given client inlines `file://` attachments is that client's
concern, and current clients may refuse. Sanitizing in calmailproc protects
the calendar regardless of which client reads it.
