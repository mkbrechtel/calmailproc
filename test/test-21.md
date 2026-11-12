# Test 21: Empty Line Inside a Folded ATTENDEE Line

## Description
This test verifies handling of an invitation whose `text/calendar` part has
an empty line in the middle of a folded content line:

```
ATTENDEE;ROLE=REQ-PARTICIPANT;PARTSTAT=NEEDS-ACTION;RSVP=TRUE;CN=…@….
<empty line>
 …:mailto:…@…
```

The empty line ends the ATTENDEE line before its `:`, and the continuation
after it has nothing to continue. calmailproc 0.2.0 fails with

```
parsing iCal data: decoding iCal data: malformed content line without ':': ATTENDEE;ROLE=REQ-PARTICIPANT;PARTSTAT=NEEDS-ACTION;RSVP=TRU...
```

The mail was sent by Horde (GU webmail, `PRODID:-//The Horde Project//Horde
iCalendar Library//EN`). It carries the same calendar twice: as an 8bit
`text/calendar` part with LF line endings, which has two such empty lines
(lines 1411 and 3646), and as a base64 `application/ics` attachment with CRLF
line endings, which is valid. Test 22 is the same Horde defect in a
DESCRIPTION.

## Scenario
1. **test-21.eml**: `METHOD:REQUEST` for a recurring series with 226 modified
   occurrences (227 VEVENTs), in a `text/calendar` part (broken) and an
   `application/ics` attachment (valid).

## Expected Behavior
The system should:
1. Not reject the mail because one copy of the calendar is broken. Either:
   - use the valid `application/ics` copy when the `text/calendar` part does
     not parse, or
   - tolerate empty lines in calendar data (RFC 5545 does not allow them, but
     they carry no data and a continuation line after one can be joined to
     the line before it)
2. Store one event: one parent and 226 occurrences

## Status
Both ways are implemented: empty lines in calendar data are ignored, and a calendar part that cannot be parsed is skipped if another one can.

## Origin
A real invitation from the archive, anonymized for the test suite: addresses,
names, UIDs and Message-IDs are replaced consistently (the same person or UID
stays the same throughout the mail), free text (subject, SUMMARY,
DESCRIPTION, LOCATION, bodies) is replaced by random characters of the same
length, and headers are reduced to those needed for parsing. The MIME
structure, the line folding, dates, recurrence rules and time zones are kept
byte for byte, and the anonymized mail fails in calmailproc 0.2.0 exactly like
the original.
