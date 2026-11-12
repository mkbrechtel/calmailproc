# Test 20: Calendar Parameters on the multipart/mixed Content-Type

## Description
This test verifies that an invitation is found in its `text/calendar` part when
the *outer* `multipart/mixed` Content-Type carries calendar parameters:

```
Content-Type: multipart/mixed;
 boundary="--==_mimepart_…";
 charset=UTF-8;
 method=PUBLISH;
 name=….ics
```

OpenProject (Ruby `mail` gem, boundaries `--==_mimepart_…`) sends its meeting
invitations like that. calmailproc 0.2.0 takes the whole multipart body as
iCalendar data and fails on its first line, the boundary:

```
parsing iCal data: decoding iCal data: malformed content line without ':': ----==_mimepart_…
```

7 mails in the archive failed like this.

## Scenario
1. **test-20.eml**: `multipart/mixed` (with `method`, `charset`, `name`) containing
   a `multipart/alternative` (text/plain, text/html) and a `text/calendar` part
   with `METHOD:REQUEST`: a recurring series as one parent and two modified
   occurrences (`RECURRENCE-ID`), SEQUENCE 0.

## Expected Behavior
The system should:
1. Treat the multipart as a multipart, whatever parameters its Content-Type has
2. Take the calendar data from the `text/calendar` part
3. Store one event: one parent and two occurrences

## Notes
- The UID has the OpenProject form with a space, so this test also needs the
  fix for test 19. With the Content-Type parameters removed, the mail fails
  with the UID error of test 19.

## Status
A multipart part is never taken as calendar data.

## Origin
A real invitation from the archive, anonymized for the test suite: addresses,
names, UIDs and Message-IDs are replaced consistently (the same person or UID
stays the same throughout the mail), free text (subject, SUMMARY,
DESCRIPTION, LOCATION, bodies) is replaced by random characters of the same
length, and headers are reduced to those needed for parsing. The MIME
structure, the line folding, dates, recurrence rules and time zones are kept
byte for byte, and the anonymized mail fails in calmailproc 0.2.0 exactly like
the original.
