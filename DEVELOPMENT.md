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
| `serde`, `serde-saphyr` | YAML config file | `src/config.rs` |
| `chrono` | DTSTAMP parsing and the current time | `src/ical/compare.rs`, `src/processor/mod.rs` |

`base64` and `chrono` are dependencies of `ureq` and `icalendar` anyway. Flag parsing, the XDG config lookup, the error type and the maildir walk are written with the standard library.

## Modules

### Email parser (`src/email.rs`)

Parses a raw email and finds its calendar part.

- `parse(raw) -> Result<Email>`: subject, from, to, date and the `ical::Event` if there is one.
- A part is a calendar part if its type is `text/calendar`, `application/ics` or `text/x-vcalendar`, or if its file name ends in `.ics`. If there are several, the last one wins.
- A calendar part that cannot be parsed is an error. A mail without a calendar part is not.
- Parts of attached messages (`message/rfc822`) are not searched.

### iCalendar (`src/ical/`)

The only module that uses the `icalendar` crate. Everything else works on its owned tree types.

- `mod.rs`: `Component` and `Property` (the calendar tree), `Event` (UID, raw data, summary, method, sequence), `decode`, `encode`, `parse_ical_data`.
- `validate.rs`: `validate` (how often a property may occur, which components may be nested), `validate_event`, `validate_uid`.
- `compare.rs`: `compare_events` (SEQUENCE first, then DTSTAMP), `parse_ical_time`.

Three things the `icalendar` crate does not do, handled in `mod.rs`:

1. Its parser accepts a content line without `:` and merges the next line into it. `decode` rejects such input before parsing.
2. Its parser tree cannot be written back as is, because TEXT values are unescaped and parameter quotes stripped on parsing. `encode` writes every property through `icalendar::Property`, which restores both.
3. It does not check cardinality. `validate` carries the rules go-ical enforced when encoding; `encode` always validates first.

Written output is semantically equal to the input but not byte-identical: parameters are sorted and quoting/escaping is normalised. Tests must therefore compare parsed calendars, not text.

### Storage (`src/storage/`)

```rust
pub trait Storage {
    fn store_event(&mut self, event: &Event) -> Result<()>;
    fn get_event(&mut self, uid: &str) -> Result<Option<Event>>;
}
```

- `caldav.rs`: `CalDavStorage`. An event is the resource `{calendar}/{UID}.ics`; reading is `GET` (404 means no event), writing is `PUT`, both with basic authentication. Summary and sequence of a stored event are parsed from the data the server returns.
- `memory.rs`: `MemoryStorage` for tests, with `list_events`, `event_count` and `delete_event` in addition.

Storage has no business logic and does not compare sequence numbers.

### Processor (`src/processor/`)

`mod.rs` holds all business logic. `Processor::process_email(raw)` returns `Result<String, Failure>`: the status message, or a `Failure` with a status message and the technical error.

- `process_event`: new events, updates, cancellations.
  1. Validate the event.
  2. No stored event: store it.
  3. The mail updates a single occurrence (a VEVENT with `RECURRENCE-ID`): merge it with `handle_recurring_event`, whatever the sequence numbers.
  4. The stored event already has occurrence updates: replace the parent event and keep them (`handle_parent_event_update`), without comparing sequence numbers.
  5. Otherwise compare with `ical::compare_events`; ignore the mail if the stored event is newer, else replace the parent event.
- `process_event_reply`: `METHOD:REPLY`, only if reply processing is enabled. `update_attendee_status` sets the replying attendee's `PARTSTAT` in the stored event. Addresses are compared case-insensitively.
- `prepare_event_for_storage`: removes `METHOD` and calls `set_schedule_agents`. Every stored event passes through here.
- `set_schedule_agents`: tells a CalDAV server with scheduling (RFC 6638) not to mail the participants again. A participant that is new on the server gets `SCHEDULE-AGENT=CLIENT`; a participant already in the stored event keeps the stored value, because changing an attendee from the default to `CLIENT` makes a server send a CANCEL if the calendar owner is the organizer (section 3.2.1.2).

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
5. `processor`: validate, compare with the stored event, merge, remove `METHOD`, set `SCHEDULE-AGENT`.
6. `storage`: `PUT` the event.
7. `stdin` or `maildir`: print the status.

## Differences from the Go version 0.1

Deliberate, each a bug in 0.1:

- A reply (`METHOD:REPLY`) now updates the attendee's status. 0.1 compared `MAILTO:` and `mailto:` case-sensitively, did not find the attendee and replaced the stored invitation with the reply.
- `VTIMEZONE` components survive updates. 0.1 dropped them whenever it replaced a parent event.
- The CalDAV storage reads summary and sequence from the stored event. 0.1 always assumed sequence 0, so an outdated update could overwrite a newer event.
- A failed lookup of the stored event (network, authentication) is an error. 0.1 treated it as "no stored event" and stored the mail's event.
- Quoted-printable calendar parts and charsets are decoded (0.1 only handled base64).
- `-process-replies=false` and `-verbose=false` override the config file.

## Testing

- `cargo test` runs everything that needs no external server:
  - unit tests next to the code (`src/**`),
  - `tests/processor.rs`: the numbered scenarios from `test/test-N.md` with the mails in `test/maildir/cur`, plus synthetic recurring event mails,
  - `tests/frontends.rs`: stdin and maildir modes,
  - `tests/end_to_end.rs`: the binary in stdin, maildir and config file mode against a minimal in-process CalDAV stand-in.
- `./test.sh` runs the same three modes against a real Xandikos server (`xandikos` must be on the `PATH`).

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
