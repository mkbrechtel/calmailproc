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

- **Processing**
  - Invitations and updates (`METHOD:REQUEST`): a newer version replaces the stored one, an older one is ignored
  - Cancellations (`METHOD:CANCEL`)
  - Attendance replies (`METHOD:REPLY`) update the attendee's status in the stored event (optional)
  - Recurring events: updates and cancellations of single occurrences are merged into the stored event
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
  -maildir string
        Path to maildir to process (will process all emails recursively)
  -pass string
        CalDAV password
  -process-replies
        Process attendance replies to update events
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

maildir:
  path: /home/alice/Mail/Invitations
  verbose: false
```

All sections and keys are optional. If `maildir.path` is set (or `-maildir` is given), the maildir is processed; otherwise an email is read from stdin. The file contains the password, so make it readable only by you (`chmod 600`).

## Storage

Each event is stored as `{calendar}/{UID}.ics` on the CalDAV server, so calendar clients that use the same calendar see the events. The `METHOD` property, which only belongs to the mail transport, is removed before storing.

Organizers and attendees of a new event are stored with `SCHEDULE-AGENT=CLIENT`. This tells a CalDAV server with scheduling support that the invitations have already been exchanged by mail, so it does not send its own. An event that is already on the server keeps the setting it has.

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
