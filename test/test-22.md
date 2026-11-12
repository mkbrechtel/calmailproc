# Test 22: Empty Line Inside a Folded DESCRIPTION

## Description
This test verifies handling of an invitation whose `text/calendar` part has
an empty line in the middle of a folded DESCRIPTION (the dial-in text of an
online meeting):

```
DESCRIPTION:…
 …
<empty line>
 …
LOCATION:…
```

calmailproc 0.2.0 fails in the iCalendar parser with

```
parsing iCal data: decoding iCal data: error: 0: at line 200, in Satisfy:
```

The mail is an export of an Exchange mailbox (LibPST, boundary
`--boundary-LibPST-iamunique-…`) of an invitation sent by Horde
(`PRODID:-//The Horde Project//Horde iCalendar Library//EN`). It carries the
calendar twice, both base64 with CRLF line endings: as an `application/ics`
attachment, which is valid, and as a `text/calendar` part with one empty line
(line 270). Without that empty line the mail is processed. Test 21 is the
same Horde defect in an ATTENDEE line.

## Scenario
1. **test-22.eml**: `METHOD:REQUEST` for a recurring series with one modified
   occurrence (2 VEVENTs) and a VTIMEZONE with many historic DAYLIGHT
   components, in an `application/ics` attachment (valid) and a `text/calendar`
   part (broken).

## Expected Behavior
The system should:
1. Not reject the mail because one copy of the calendar is broken (see test 21
   for the two ways)
2. Store one event: one parent and one occurrence

## Notes
- The error message names neither the empty line nor the part; a message that
  does would have made this much quicker to find.

## Status
Both ways are implemented, see test 21. Parse errors now name the part and show the offending line.

## Origin
A real invitation from the archive, anonymized for the test suite: addresses,
names, UIDs and Message-IDs are replaced consistently (the same person or UID
stays the same throughout the mail), free text (subject, SUMMARY,
DESCRIPTION, LOCATION, bodies) is replaced by random characters of the same
length, and headers are reduced to those needed for parsing. The MIME
structure, the line folding, dates, recurrence rules and time zones are kept
byte for byte, and the anonymized mail fails in calmailproc 0.2.0 exactly like
the original.
