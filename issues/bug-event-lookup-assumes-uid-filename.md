# Title: Stored events are only found if they are named {UID}.ics

**Type:** Bug

**Status:** Open

## Description

calmailproc reads and writes an event at `{calendar}/{UID}.ics`. That is only
where calmailproc itself puts events. CalDAV does not tie the resource name to
the UID, and other writers choose their own names:

- a calendar client that created the event (some use the UID, some a random
  name),
- the server, when it delivers an invitation into the calendar itself
  (RFC 6638 scheduling).

For such an event the `GET` returns 404, calmailproc concludes that there is
no stored event, and tries to create one with `PUT {UID}.ics`. CalDAV requires
the UID to be unique within a calendar (RFC 4791, section 5.3.2.1,
`CALDAV:no-uid-conflict`), so the server either rejects the `PUT` or, if it
does not enforce the rule, ends up with two resources for one event.

Either way the existing event is never updated: no cancellation, no new time,
no attendee status.

Thunderbird and Evolution both look an event up by UID, wherever it is stored,
and write back to the resource they found.

## Steps to Reproduce

1. Create an event in a CalDAV calendar with a client that does not name the
   resource after the UID, or let a scheduling server deliver an invitation.
2. Process a mail for the same UID with calmailproc (an update, a
   cancellation or a reply).

## Expected Behavior

The existing event is found and updated in place.

## Actual Behavior

`GET {UID}.ics` returns 404. calmailproc treats the event as new and sends
`PUT {UID}.ics`, which fails with a UID conflict or creates a duplicate.

## Proposed Solution

In `CalDavStorage::get_event()` (`src/storage/caldav.rs`):

1. Try `GET {UID}.ics` first; this is the common case for events calmailproc
   stored itself and needs no query.
2. On 404, send a `REPORT` with a `calendar-query` filtered by UID
   (RFC 4791, section 7.8.6: `<C:prop-filter name="UID"><C:text-match>`),
   asking for `getetag` and `calendar-data`.
3. Remember the resource path (href) that was found. `store_event()` must
   write to that path, not to `{UID}.ics`.

This needs the `Storage` trait to carry the location from `get_event` to
`store_event`, for example as an opaque field on `Event` or as a value
returned by `get_event` and passed back. The same value can hold the ETag
(see `improvement-conditional-put-with-etag.md`).

Parsing the multistatus response needs an XML parser; `quick-xml` or
`roxmltree` were considered when the dependencies were chosen.

The fake server in `tests/end_to_end.rs` has to learn `REPORT`. `test.sh`
should store one event under a different name with `curl` before running, to
cover this against Xandikos.

## Additional Context

- Found by comparing with Thunderbird (`calItipUtils.sys.mjs`,
  `ItipItemFinder`) and evolution-data-server (`e-cal-backend-caldav.c`,
  which keeps the href of every resource).
- This matters most on a groupware server that the user also uses with a
  calendar client. On a calendar that only calmailproc writes to, it does
  not occur.
