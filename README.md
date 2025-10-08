[![Build Status](https://github.com/mkbrechtel/calmailproc/actions/workflows/test-build.yml/badge.svg)](https://github.com/mkbrechtel/calmailproc/actions/workflows/test-build.yml)

# calmailproc

A calendar mail processor that extracts calendar event information from emails containing iCalendar invitation events and stores them in various calendar storage formats.

STATUS: WIP with major issues still.

## Features

- **Input Options**:
  - Process email from stdin (default)
  - Process email files from maildir folders (with recursive subfolder support)
  
- **Storage Options**:
  - Store calendar events via CalDAV server
  - Support for XDG-based YAML configuration

- **Processing Features**:
  - Parse iCalendar invitation data from email attachments
  - Handle invitation updates (METHOD:REQUEST)
  - Process attendance replies (METHOD:REPLY) to update event status
  - Support for recurring events and updates to specific occurrences
  
- **Output Options**:
  - Plain text output showing event details
  - Configurable verbosity levels for debugging
  
## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/mkbrechtel/calmailproc.git
cd calmailproc

# Build the application
go build -o calmailproc
```

### Using Go Install

```bash
go install github.com/mkbrechtel/calmailproc@latest
```

## Usage

### Process a single email

```bash
# Process an email file and display information in plain text
cat email.eml | calmailproc

# Specify CalDAV server URL
cat email.eml | calmailproc -caldav https://caldav.example.com/user/calendar/
```

### Process a maildir

```bash
# Process all emails in a maildir (recursively)
calmailproc -maildir ~/Mail/MyFolder

# With verbose output
calmailproc -maildir ~/Mail/MyFolder -verbose

# Using a specific CalDAV server
calmailproc -maildir ~/Mail/MyFolder -caldav https://caldav.example.com/user/calendar/
```

### Command Line Options

```
Usage of calmailproc:
  -caldav string
        CalDAV server URL for calendar storage
  -maildir string
        Path to maildir to process (will process all emails recursively)
  -process-replies
        Process attendance replies to update events (default true)
  -verbose
        Enable verbose logging output
```

### Integration with mail systems

The tool is designed to be used in standard Unix mail pipelines. For example:

```bash
# Process incoming mail with procmail
:0c
* ^Content-Type:.*text/calendar
| calmailproc -caldav https://caldav.example.com/user/calendar/ > /path/to/logs/calendar.log
```

## Configuration

calmailproc supports XDG-based configuration via YAML file at `~/.config/calmailproc/config.yaml`:

```yaml
caldav:
  url: https://caldav.example.com/user/calendar/
  username: your-username
  # Password can be provided via environment variable CALDAV_PASSWORD
```

## Storage Format

### CalDAV

Calendar events are stored on a CalDAV server, allowing synchronization across multiple devices and integration with standard calendar applications.

## Development

### Running Tests

```bash
go test ./...
```

### Formatting and Linting

```bash
go fmt ./...
golangci-lint run
```

## Contributing

### Reporting Issues

Issues should be submitted as markdown files in the `issues/` directory via pull requests to the `main` branch. This approach allows for better tracking and documentation of problems and feature requests.

See [issues/README.md](issues/README.md) for detailed instructions on how to report and track issues.

## GitHub Workflows

This project uses GitHub Actions for continuous integration:

- **Test and Build**: Ensures code quality by running tests and verifying that the code builds successfully across different Go versions
- **Release**: Automatically creates binaries for multiple platforms when a new tag is pushed

## License

Apache License 2.0
