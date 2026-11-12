# Test 19: UID With a Space

## Description
This test verifies that an invitation whose UID contains a space is
processed. RFC 5545 defines UID as a TEXT value, so a space (and in general
any character but control characters) is allowed.

OpenProject (meeting module, `PRODID:-//OpenProject GmbH//…//Meeting//EN`)
generates UIDs of the form `<Word> <Word>-<host>-<name>-<n>`, with a space.
In the archive this was the most frequent failure: 46 mails (28 REQUEST,
12 REPLY, 6 CANCEL) were rejected with

```
invalid UID: UID contains invalid characters (only alphanumeric and @-_.:+ allowed)
```

## Scenario
1. **test-19.eml**: `METHOD:REQUEST` for one occurrence (`RECURRENCE-ID`) of an
   OpenProject meeting, SEQUENCE 1, UID with a space.

## Expected Behavior
The system should:
1. Accept the UID as it is
2. Store one event under that UID

## Notes
- The CalDAV storage builds the resource path from the UID, so the UID has to
  be percent-encoded there (`%20`), or mapped to a safe resource name.
- Test 20 has the same UID form and needs this fix as well.

## Origin
A real invitation from the archive, anonymized for the test suite: addresses,
names, UIDs and Message-IDs are replaced consistently (the same person or UID
stays the same throughout the mail), free text (subject, SUMMARY,
DESCRIPTION, LOCATION, bodies) is replaced by random characters of the same
length, and headers are reduced to those needed for parsing. The MIME
structure, the line folding, dates, recurrence rules and time zones are kept
byte for byte, and the anonymized mail fails in calmailproc 0.2.0 exactly like
the original.
