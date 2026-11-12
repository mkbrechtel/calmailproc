# Test 23: Occurrence Without DTSTAMP

## Description
This test verifies that an invitation is stored when one of its VEVENTs has no
DTSTAMP. Outlook (`PRODID:-//Microsoft Corporation//Outlook 16.0 MIMEDIR//EN`)
sends the modified occurrences of a series with only DTSTART, DTEND,
RECURRENCE-ID, UID and an alarm. The mail is parsed, but storing it fails:

```
validation error for event …: encoding calendar: failed to encode VEVENT: want exactly one DTSTAMP property, got 0
```

RFC 5545 requires DTSTAMP in every VEVENT of an iTIP message, so the input is
invalid, but the data is otherwise complete and the error comes only after
calmailproc decided to store it.

## Scenario
1. **test-23.eml**: single-part `text/calendar` mail, `METHOD:REQUEST`, a weekly
   series (RRULE with COUNT, an EXDATE and an RDATE, SEQUENCE 10) with a
   VTIMEZONE `W. Europe Standard Time`, and one modified occurrence
   (`RECURRENCE-ID`) without DTSTAMP.

## Expected Behavior
The system should:
1. Add a DTSTAMP to a VEVENT that has none, from the series' DTSTAMP (or the
   current time), before validating and encoding
2. Store one event: one parent and one occurrence, every VEVENT with exactly
   one DTSTAMP

## Status
A missing DTSTAMP is taken from another event in the mail, else from the date of the mail.

## Origin
A real invitation from the archive, anonymized for the test suite: addresses,
names, UIDs and Message-IDs are replaced consistently (the same person or UID
stays the same throughout the mail), free text (subject, SUMMARY,
DESCRIPTION, LOCATION, bodies) is replaced by random characters of the same
length, and headers are reduced to those needed for parsing. The MIME
structure, the line folding, dates, recurrence rules and time zones are kept
byte for byte, and the anonymized mail fails in calmailproc 0.2.0 exactly like
the original.
