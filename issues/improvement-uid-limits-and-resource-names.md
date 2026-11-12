# Title: Decide which UIDs are accepted and how they become resource names

**Type:** Improvement

**Status:** Open

## Description

Since test 19, calmailproc accepts almost any UID and stores a new event as
`{calendar}/{percent-encoded UID}.ics`. Two questions are open: which UIDs
should be accepted at all, and how a UID with unusual characters should be
turned into a resource name.

### What the specifications say

They contradict each other, and what they allow is looser than is useful.

- RFC 5545, section 3.8.4.7: `uid = "UID" uidparam ":" text CRLF`. UID is a
  TEXT value (section 3.3.11). That allows every character except control
  characters (`%x00-08`, `%x0A-1F`, `%x7F`). Tab is allowed. Any non-ASCII
  character is allowed. A newline is allowed in its escaped form `\n`, as
  are `\;`, `\,` and `\\`.
- RFC 5545 gives no maximum length: "Implementations MUST be able to receive
  and persist values of at least 255 octets for this property". That is a
  minimum for receivers, not a limit for senders.
- RFC 5545 recommends building the UID as `something@host`.
- RFC 7986, section 5.3, forbids that recommendation ("MUST NOT include any
  data that might identify a user, host, domain"), recommends a UUID, and
  says that any other form "MUST have a length less than 255 octets and MUST
  conform to the iana-token ABNF syntax", which is letters, digits and `-`
  only. So `@` and `.`, which RFC 5545 itself recommended, are not allowed.

Senders follow neither: OpenProject sends spaces (test 19), Horde sends
`...@host`, Outlook sends 112 hexadecimal characters.

### What calmailproc does now

`validate_uid()` in `src/ical/validate.rs` rejects a UID that is empty,
longer than 255 bytes, or contains a control character. This rejects
newlines (the parser has already turned an escaped `\n` into a real one) and
tabs. It accepts everything else: spaces, slashes, quotes, non-ASCII.

`CalDavStorage::event_url()` in `src/storage/caldav.rs` percent-encodes every
character of the UID except letters, digits and `- _ . ~ @ : +`.

### What is not handled

- **Slash.** A UID with `/` becomes `%2F` in the path. Servers and proxies
  may decode it and treat it as a path separator, or reject it; Apache
  rejects encoded slashes by default. evolution-data-server avoids this on
  purpose (`ecb_caldav_uid_to_uri()` in `e-cal-backend-caldav.c`): "UIDs with
  forward slashes can cause trouble, because the destination server can
  consider them as a path delimiter. Double-encode the URL doesn't always
  work". It uses the SHA-1 hash of such a UID as the name.
- **Name length.** Percent-encoding triples every encoded byte, so a
  255-byte UID can become a name of more than 760 characters. Servers that
  store resources as files, such as Xandikos, are limited to 255 bytes per
  file name.
- **Leading and trailing whitespace.** Accepted as it is. A server that
  trims the value would then hold a different UID from the one calmailproc
  looks up.
- **Case.** UIDs are compared exactly. A server on a case-insensitive file
  system maps two UIDs that differ only in case to one resource.
- **Invisible characters.** Zero-width and bidirectional formatting
  characters are not control characters and pass the check.

For comparison, Thunderbird replaces every character outside
`a-z A-Z 0-9 _ - .` with `_` in the resource name (`getItemLocationPath()`
in `CalDavCalendar.sys.mjs`).

## Expected Behavior

- It is defined and documented which UIDs calmailproc accepts.
- Every accepted UID gives a resource name that a CalDAV server will take:
  no encoded slash, not longer than a file name may be.
- Resource names of events that are already stored do not change.

## Actual Behavior

See "What is not handled" above. None of these cases has been seen in real
mail so far: the import of 1835 mails had UIDs with spaces, but none with a
slash and none near the length limit.

## Proposed Solution

An event is found by a query for its UID if it is not at the expected name
(`find_by_uid()`), so the resource name does not have to encode the UID
reversibly. That allows a simple rule.

1. **Validation.** Keep the current checks. In addition reject a UID with
   leading or trailing whitespace, or trim it; decide which. Decide whether
   invisible formatting characters are rejected too.

2. **Resource name.** Use the UID itself as the name only if it is plain:
   letters, digits and `@ . _ + : -`, and at most about 200 bytes. These are
   the names calmailproc has written since 0.1, so nothing that is stored
   changes.

3. **Every other UID.** Choose one:

   a. A hash of the UID as the name, as Evolution does for UIDs with a
      slash. Solves slash, length and unusual characters at once, and two
      UIDs cannot get the same name. Needs a hash that is stable across
      versions, so one more dependency (`sha2` or similar); the hasher of
      the standard library is not guaranteed to be stable.

   b. Replace every other character with `_`, as Thunderbird does, and cut
      the name to a maximum length. No dependency. Two UIDs can then get the
      same name ("a b" and "a_b"). The conditional write (`If-None-Match: *`)
      turns that into an error instead of one event overwriting the other,
      but the second event cannot be stored without a further rule, such as
      a numbered suffix.

   c. Keep percent-encoding and only replace `/`. Smallest change, but does
      not solve the length.

Events stored with a percent-encoded name by the current version (UIDs with
spaces, as in test 19) are still found: the lookup by name misses, the query
by UID finds them, and they are updated where they are.

Tests to add in `src/storage/caldav.rs` and `src/ical/validate.rs`: a UID
with a slash, a 255-byte UID of non-ASCII characters, a UID with surrounding
whitespace, two UIDs that differ only in case, and that a plain UID still
gives `{UID}.ics`.

## Additional Context

- RFC 5545, sections 3.3.11 (TEXT) and 3.8.4.7 (UID); RFC 7986, section 5.3.
- RFC 4791, section 5.3.2.1: a UID must be unique within a calendar
  collection (`CALDAV:no-uid-conflict`). It says nothing about the
  characters of a UID or about resource names.
- Related, resolved: `archive/bug-event-lookup-assumes-uid-filename.md`,
  which added the lookup by UID that makes a non-reversible name possible.
