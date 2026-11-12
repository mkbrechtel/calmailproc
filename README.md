[![Build Status](https://github.com/mkbrechtel/calmailproc/actions/workflows/test-build.yml/badge.svg)](https://github.com/mkbrechtel/calmailproc/actions/workflows/test-build.yml)

# calmailproc

A calendar mail processor: it extracts iCalendar invitations from emails and stores them as events on a CalDAV server.

STATUS: WIP. Version 0.2 is a rewrite in Rust of the Go version 0.1.

## Features

- **Input**
  - A single email from stdin (default)
  - All emails in a maildir, including subfolders

- **Storage**
  - Events are stored on a CalDAV server, one resource per event UID
  - An event that is already in the calendar is found by its UID, whatever its resource is called, and updated in place
  - A change made by someone else between reading and writing is not overwritten

- **Processing**
  - Invitations and updates (`METHOD:REQUEST`): a newer version replaces the stored one, an older one is ignored
  - Cancellations (`METHOD:CANCEL`): the event is kept and marked as cancelled, or deleted if configured
  - Attendance replies (`METHOD:REPLY`) update the status of the replying attendee in the stored event (optional)
  - Recurring events: updates and cancellations of single occurrences are merged into the stored event
  - The result does not depend on the order in which the mails for an event are processed
  - Invalid calendar data is rejected with an error

- **Output**
  - One status line per processed email
  - Verbose logging for maildir runs

## Installation

Requires Rust 1.98 or newer (the `mail-parser` dependency sets this). Distribution packages are often older; install a current toolchain with [rustup](https://rustup.rs).

```bash
git clone https://github.com/mkbrechtel/calmailproc.git
cd calmailproc
cargo build --release
# the binary is target/release/calmailproc
```

Or install directly:

```bash
cargo install --git https://github.com/mkbrechtel/calmailproc
```

## Usage

### Process a single email

```bash
calmailproc -url https://caldav.example.com -user alice -pass secret \
    -calendar /alice/calendars/invitations/ < email.eml
```

The exit code is 0 if the email was processed (also if it contains no calendar data) and 1 on an error, for example invalid calendar data or an unreachable server.

### Process a maildir

```bash
calmailproc -maildir ~/Mail/Invitations -verbose \
    -url https://caldav.example.com -user alice -pass secret \
    -calendar /alice/calendars/invitations/
```

The `new` and `cur` folders of the maildir and of all its subfolders are processed. An email that cannot be processed is reported as a warning and does not stop the run.

### Command line options

```
Usage of calmailproc:
  -calendar string
        CalDAV calendar path (e.g., /calendar/)
  -delete-cancelled
        Delete cancelled events and occurrences instead of marking them
  -drop-alarms
        Remove alarms from events before storing them
  -ignore-unknown-cancellations
        Ignore a cancellation if its event is not stored
  -maildir string
        Path to maildir to process (will process all emails recursively)
  -pass string
        CalDAV password
  -pass-cmd string
        Shell command that prints the CalDAV password (instead of -pass)
  -process-replies
        Process attendance replies to update events
  -process-replies-without-event
        Store a reply as an event if its event is not stored
  -url string
        CalDAV server URL (e.g., http://localhost:5232)
  -user string
        CalDAV username
  -verbose
        Enable verbose logging output
  -version
        Print the version and exit
```

Flags can be written as `-url x`, `--url x` or `-url=x`. Boolean flags take a value only in the form `-verbose=false`.

### Integration with mail systems

The tool is designed for standard Unix mail pipelines. For example with procmail:

```
:0c
* ^Content-Type:.*(text/calendar|multipart)
| calmailproc >> /path/to/logs/calendar.log 2>&1
```

Here the CalDAV settings come from the config file.

## Configuration

Settings that are not given as flags are read from `$XDG_CONFIG_HOME/calmailproc/config.yaml` (by default `~/.config/calmailproc/config.yaml`):

```yaml
webdav:
  url: https://caldav.example.com
  user: alice
  pass: secret
  calendar: /alice/calendars/invitations/

processor:
  process_replies: true
  process_replies_without_event: false
  ignore_unknown_cancellations: false
  delete_cancelled: false
  drop_alarms: false

maildir:
  path: /home/alice/Mail/Invitations
  verbose: false
```

All sections and keys are optional. If `maildir.path` is set (or `-maildir` is given), the maildir is processed; otherwise an email is read from stdin.

### Password

The password does not have to be in the config file. With `pass_cmd` in place of `pass`, calmailproc runs a shell command and uses its output as the password, like `PassCmd` in mbsync:

```yaml
webdav:
  url: https://caldav.example.com
  user: alice
  pass_cmd: pass show calendar/alice
  calendar: /alice/calendars/invitations/
```

Other examples are `secret-tool lookup service caldav user alice` or `gpg --quiet --decrypt ~/.config/calmailproc/password.gpg`.

- The command is run by `sh -c` each time calmailproc starts, and must print exactly one line: the password. A trailing line break is removed; nothing else is.
- If the command fails, prints nothing or prints several lines, calmailproc stops with an error before it contacts the server.
- The command cannot read standard input, which carries the mail. A command that asks for a passphrase has to use the terminal or an agent; when calmailproc runs unattended from a mail filter there is no terminal, so the secret store must already be unlocked.
- Use either `pass` or `pass_cmd`, not both. On the command line, `-pass` or `-pass-cmd` replaces whichever of the two the config file has.

If you do put the password into the file as `pass`, make the file readable only by you (`chmod 600`). A password given as `-pass` on the command line is visible to other users of the machine in the process list.

## How mails are processed

Each event in a mail is handled by its UID. If the calendar has no event with that UID, the event is stored. Otherwise the mail is merged into the stored event:

- The event itself and each changed occurrence of a recurring event are compared with their stored version by `SEQUENCE` and then `DTSTAMP`. The stored version stays if it is newer, so an old mail never undoes a newer one, whatever the order of processing.
- A cancellation marks the event or the occurrence with `STATUS:CANCELLED`. The calendar then shows what was cancelled. With `delete_cancelled`, the event is deleted, or the occurrence removed from the series, instead.
- A cancellation for an event that is not in the calendar is stored as a cancelled event, so that the event stays cancelled if its invitation is processed later. `ignore_unknown_cancellations` turns that off.
- A reply sets the status of the replying attendee and nothing else. Someone who was not invited is added as an attendee. The comment of a reply is stored with the attendee (`X-RESPONSE-COMMENT`).
- A reply for an event that is not in the calendar is ignored. `process_replies_without_event` stores it as an event anyway; use that only for a single mail you want in the calendar.

What cannot be made independent of the order: a reply that is processed before its invitation is lost.

## Storage

A new event is stored as `{calendar}/{UID}.ics`, with the UID percent-encoded. An event that something else put into the calendar under another name is found with a query for its UID and updated where it is.

Before an event is stored:

- The `METHOD` property, which only belongs to the mail transport, is removed.
- Organizers and attendees that are new on the server get `SCHEDULE-AGENT=CLIENT`. This tells a CalDAV server with scheduling support that the invitations have already been exchanged by mail, so it does not send its own. Participants of an event that is already on the server keep the setting they have.
- Attachments that refer to local files (`ATTACH:file:...`) are made unusable, and with `drop_alarms` the alarms of the sender are removed.

## Development

```bash
cargo test                      # unit, integration and end-to-end tests
cargo fmt                       # formatting
cargo clippy --all-targets      # linting
./test.sh                       # end-to-end test against a Xandikos CalDAV server
```

See [DEVELOPMENT.md](DEVELOPMENT.md) for the architecture.

## Contributing

### Reporting Issues

Issues are submitted as markdown files in the `issues/` directory via pull requests to the `main` branch. See [issues/README.md](issues/README.md) for details.

## GitHub Workflows

- **Test and Build**: checks formatting, runs clippy and the tests, and builds a release binary
- **Release**: builds binaries for Linux, macOS and Windows when a `v*` tag is pushed

## License

Apache License 2.0
