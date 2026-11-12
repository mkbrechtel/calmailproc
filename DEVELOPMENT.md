# calmailproc - Development Guide

## Overview

calmailproc is a Rust application that processes emails containing calendar data (iCalendar/ICS), extracts the calendar event and stores it on a CalDAV server. Version 0.2 is a port of the Go version 0.1; the module structure and the status messages were kept.

## Dependencies

Each protocol is handled by one crate; everything else uses the standard library.

| Crate | Used for | Used in |
|---|---|---|
| `mail-parser` | Email and MIME parsing, transfer encodings, charsets | `src/email.rs` |
| `icalendar` | iCalendar parsing and writing | `src/ical/mod.rs` |
| `ureq` (with `rustls`) | HTTP for CalDAV | `src/storage/caldav.rs` |
| `base64` | Basic authentication header | `src/storage/caldav.rs` |
| `percent-encoding` | Resource names from UIDs | `src/storage/caldav.rs` |
| `roxmltree` | Reading the result of a calendar query | `src/storage/caldav.rs` |
| `serde`, `serde-saphyr` | YAML config file | `src/config.rs` |
| `chrono` | DTSTAMP parsing and the current time | `src/ical/compare.rs`, `src/processor/mod.rs` |

`base64`, `percent-encoding` and `chrono` are dependencies of `ureq` and `icalendar` anyway. Flag parsing, the XDG config lookup, the error type and the maildir walk are written with the standard library.

## Modules

### Email parser (`src/email.rs`)

Parses a raw email and finds its calendar part.

- `parse(raw) -> Result<Email>`: subject, from, to, date and the `ical::Event` if there is one.
- A part is a calendar part if its type is `text/calendar`, `application/ics` or `text/x-vcalendar`, or if its file name ends in `.ics`. A multipart is never a calendar part, whatever its Content-Type says (test 20).
- Mails often carry the same calendar twice. The last part that can be parsed wins. A part that cannot be parsed only matters if none can: then it is an error, which names the part (tests 21, 22).
- A mail without a calendar part is not an error.
- Parts of attached messages (`message/rfc822`) are not searched.

### iCalendar (`src/ical/`)

The only module that uses the `icalendar` crate. Everything else works on its owned tree types.

- `mod.rs`: `Component` and `Property` (the calendar tree), `Event` (UID, raw data, summary, method, sequence), `decode`, `encode`, `parse_ical_data`.
- `validate.rs`: `validate` (how often a property may occur, which components may be nested), `validate_event`, `validate_uid` (any text without control characters, at most 255 bytes).
- `compare.rs`: `compare_components` and `compare_events` (SEQUENCE first, then DTSTAMP), `parse_ical_time`.

Three things the `icalendar` crate does not do, handled in `mod.rs`:

1. Its parser accepts a content line without `:` and merges the next line into it, and it splits a property name at a space. `decode` checks the shape of every content line before parsing.
2. Its parser tree cannot be written back as is, because TEXT values are unescaped and parameter quotes stripped on parsing. `encode` writes every property through `icalendar::Property`, which restores both.
3. It does not check cardinality. `validate` carries the rules go-ical enforced when encoding; `encode` always validates first.

`decode` ignores empty lines, which Horde puts into folded lines (tests 21, 22), and accepts mixed line endings. A parser error is reported with the text of the line it stopped at.

Written output is semantically equal to the input but not byte-identical: parameters are sorted and quoting/escaping is normalised. Tests must therefore compare parsed calendars, not text.

### Storage (`src/storage/`)

```rust
pub trait Storage {
    fn get_event(&mut self, uid: &str) -> Result<Option<Stored>>;
    fn store_event(&mut self, event: &Event, replaces: Option<&Version>) -> Result<(), StoreError>;
    fn delete_event(&mut self, version: &Version) -> Result<(), StoreError>;
}
```

`get_event` returns the event with a `Version`: where it is stored and which version was read. Writing takes that `Version` back, or `None` for an event that must be new. If the stored event is no longer that version, the result is `StoreError::Conflict`.

- `caldav.rs`: `CalDavStorage`.
  - Reading: `GET {calendar}/{percent-encoded UID}.ics`. On 404, a `REPORT` with a `calendar-query` for the UID finds an event stored under any other name.
  - Writing: `PUT` to the location the event was read from, with `If-Match` and its ETag; a new event with `If-None-Match: *`. `DELETE` with `If-Match`.
  - A failed condition (412) is a conflict. A 412 whose body names a CalDAV precondition (`valid-calendar-data`, ...) is an error; Xandikos answers that way for invalid data.
  - Errors include the response body, which is where a server says what it objects to.
  - Summary and sequence of a stored event are parsed from the data the server returns.
- `memory.rs`: `MemoryStorage` for tests. It checks versions like a server checks ETags, and has `list_events`, `event_count`, `event`, `put` and `conflicts_to_inject` for the tests.

Storage has no business logic and does not compare sequence numbers.

### Processor (`src/processor/`)

`mod.rs` holds all business logic. `Processor::process_email(raw)` returns `Result<String, Failure>`: the status message, or a `Failure` with a status message and the technical error.

A mail's calendar is split by UID (`split_by_uid`); each UID is one stored event and is processed on its own (`process_calendar`):

1. `normalize`: add a missing `DTSTAMP` (from another event of the mail, else the mail's date; test 23), drop alarms if configured, and `sanitize` (neutralise `ATTACH:file:`, remove `SCHEDULE-FORCE-SEND` and `SCHEDULE-STATUS`).
2. Validate.
3. Read the stored event, decide what to do (`apply_update` or `apply_reply`), and carry it out (`perform`). On a conflict from storage, repeat this step, up to three times.

`apply_update` (REQUEST, CANCEL, PUBLISH, anything that carries event data):

- No stored event: store the mail's event. A cancellation is stored as a cancelled event unless `ignore_unknown_cancellations` or `delete_cancelled` is set.
- Otherwise go through every VEVENT of the mail. The parent (no `RECURRENCE-ID`) is compared with the stored parent, an occurrence with the stored occurrence of the same `RECURRENCE-ID` (`ical::compare_components`). If the stored one is newer, the component is skipped.
- A new parent brings the mail's calendar properties and time zones; stored occurrences stay. A new occurrence replaces the stored one or is added.
- A cancellation sets `STATUS:CANCELLED` on the component. With `delete_cancelled` the event is deleted, or the occurrence removed and an `EXDATE` added to the parent.
- If nothing was applied: `Not processing older event ...`.

`apply_reply` (REPLY, only with `process_replies`):

- No stored event: ignored, or stored with `force`.
- Otherwise the reply sets `PARTSTAT` of the replying attendee in the VEVENT it refers to (the parent, or the occurrence named by `RECURRENCE-ID`). Addresses are compared without case. An attendee who is not in the event is added. The reply's `COMMENT` becomes the parameter `X-RESPONSE-COMMENT` of the attendee.

`prepare_event_for_storage` removes `METHOD` and calls `set_schedule_agents`. Every stored event passes through here.

`set_schedule_agents` tells a CalDAV server with scheduling (RFC 6638) not to mail the participants again. A participant that is new on the server gets `SCHEDULE-AGENT=CLIENT`; a participant already in the stored event keeps the stored value, because changing an attendee from the default to `CLIENT` makes a server send a CANCEL if the calendar owner is the organizer (section 3.2.1.2).

The two input modes:

- `stdin.rs`: `process` reads one email from stdin and prints the status; an error becomes exit code 1.
- `maildir.rs`: `process` walks `new` and `cur` of a maildir and of all subfolders, in name order. A failing email is a warning on stderr; the run continues.

### CLI and config (`src/cli.rs`, `src/config.rs`)

- `cli::parse_flags` parses Go-style flags (`-url x`, `--url x`, `-url=x`) so that recipes written for 0.1 keep working.
- `config::load` reads `$XDG_CONFIG_HOME/calmailproc/config.yaml`; a missing file is an empty config, a broken one a warning.
- `cli::merge` puts flags on top of the config file; `cli::run` requires all four CalDAV settings, then runs maildir mode if a maildir is set and stdin mode otherwise.

### Errors (`src/error.rs`)

One `Error` type holding a message. `result.context("parsing email")?` prefixes the cause, giving chains such as `error processing stdin: processing stdin: validation error for event X: encoding calendar: failed to encode VEVENT: want at most one URL property, got 2`. Only `cli::main`, `stdin` and `maildir` print.

## Data flow

1. `cli`: parse flags, load config, create storage and processor.
2. `stdin` or `maildir`: read the raw email.
3. `email`: parse the mail, find the calendar part.
4. `ical`: parse the calendar into an `Event`.
5. `processor`: per UID: repair and sanitize, validate, read the stored event, merge each component by version, remove `METHOD`, set `SCHEDULE-AGENT`.
6. `storage`: `PUT` the event where it was found, if it is still the version that was read.
7. `stdin` or `maildir`: print the status.

## Differences from the Go version 0.1

Bugs of 0.1 that are fixed:

- A reply (`METHOD:REPLY`) only changes its own attendee. 0.1 compared `MAILTO:` and `mailto:` case-sensitively, did not find the attendee and replaced the stored invitation with the reply.
- `VTIMEZONE` components survive updates. 0.1 dropped them whenever it replaced a parent event.
- The CalDAV storage reads summary and sequence from the stored event. 0.1 always assumed sequence 0, so an outdated update could overwrite a newer event.
- A failed lookup of the stored event (network, authentication) is an error. 0.1 treated it as "no stored event" and stored the mail's event.
- Quoted-printable calendar parts and charsets are decoded (0.1 only handled base64).
- Every event of a mail is processed, and each occurrence of a series is compared with its own stored version. 0.1 used the first event only and let the last processed mail win within a series.
- UIDs with spaces, calendar parameters on a multipart, empty lines in calendar data and a missing `DTSTAMP` no longer make a mail fail (tests 19 to 23).

Changed behaviour:

- A reply for an event that is not stored is ignored unless `-force` is given. 0.1 stored the reply as an event.
- Events are found by UID under any resource name, written conditionally, and get `SCHEDULE-AGENT=CLIENT` when they are new on the server.
- `-process-replies=false` and `-verbose=false` override the config file.
- New options: `-force`, `-ignore-unknown-cancellations`, `-delete-cancelled`, `-drop-alarms`.

## Testing

- `cargo test` runs everything that needs no external server:
  - unit tests next to the code (`src/**`),
  - `tests/processor.rs`: the numbered scenarios from `test/test-N.md` with the mails in `test/maildir/cur`, plus synthetic mails for recurring events, replies, mails with several events and the options,
  - `tests/order.rs`: the result must not depend on the order in which mails are processed, and processing them again must change nothing,
  - `tests/frontends.rs`: stdin and maildir modes,
  - `tests/end_to_end.rs`: the binary in stdin, maildir and config file mode against a minimal in-process CalDAV stand-in that implements ETags, conditional writes and the UID query.
- `./test.sh` runs the same three modes against a real Xandikos server (`xandikos` must be on the `PATH`), and checks that an event stored under another name is found and updated in place.

Test mails 19 to 23 are anonymized mails from a real archive; `test-21.eml` is 660 KB.

## Development workflow

```bash
cargo test
cargo fmt
cargo clippy --all-targets
./test.sh
```

## Developer tools

`examples/maildecoder.rs` shows the structure of an email and decodes its attachments:

```bash
# Print headers and all text parts, including calendar attachments
cargo run --example maildecoder -- -email=test/maildir/cur/test-05-2.eml

# Save all parts to files
cargo run --example maildecoder -- -email=mail.eml -extract-all -out=attachments
```

Library documentation: `cargo doc --open`.

## Important notes for AI agents

After making changes, ensure `cargo test`, `cargo fmt --check` and `cargo clippy --all-targets` pass.
