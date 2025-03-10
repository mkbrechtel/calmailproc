[![Build Status](https://github.com/mkbrechtel/calmailproc/actions/workflows/test-build.yml/badge.svg)](https://github.com/mkbrechtel/calmailproc/actions/workflows/test-build.yml)

# calmailproc

A calendar mail processor that extracts calendar event information from emails containing iCalendar invitation events and stores them in various calendar storage formats.

## Features

- **Input Options**:
  - Process email from stdin (default)
  - Process email files from maildir folders (with recursive subfolder support)
  
- **Storage Options**:
  - Store calendar events in vdir format (compatible with vdirsyncer)
  - Store calendar events in a single iCalendar file
  - Default storage uses ~/.calendar directory

- **Processing Features**:
  - Parse iCalendar invitation data from email attachments
  - Handle invitation updates (METHOD:REQUEST)
  - Process attendance replies (METHOD:REPLY) to update event status
  - Support for recurring events and updates to specific occurrences
  
- **Output Options**:
  - Plain text output showing event details
  - JSON output for integration with other tools
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

# Process and store the calendar event
cat email.eml | calmailproc -store

# Process and output in JSON format
cat email.eml | calmailproc -json

# Specify storage location (vdir format)
cat email.eml | calmailproc -store -vdir ~/.calendar/events
```

### Process a maildir

```bash
# Process all emails in a maildir (recursively)
calmailproc -maildir ~/Mail/MyFolder -store

# With verbose output
calmailproc -maildir ~/Mail/MyFolder -store -verbose

# Using a specific storage location
calmailproc -maildir ~/Mail/MyFolder -store -vdir ~/.calendar/invitations
```

### Command Line Options

```
Usage of calmailproc:
  -icalfile string
        Path to single iCalendar file storage
  -json
        Output in JSON format
  -maildir string
        Path to maildir to process (will process all emails recursively)
  -process-replies
        Process attendance replies to update events (default true)
  -store
        Store calendar event if found
  -vdir string
        Path to vdir storage directory
  -verbose
        Enable verbose logging output
```

### Integration with mail systems

The tool is designed to be used in standard Unix mail pipelines. For example:

```bash
# Process incoming mail with procmail
:0c
* ^Content-Type:.*text/calendar
| calmailproc -store -vdir ~/.calendar/invitations > /path/to/logs/calendar.log
```

## Storage Formats

### vdir

The vdir format stores each calendar event as a separate file in a directory structure, making it compatible with vdirsyncer and other calendar tools. Each event is stored in a file named with the event's UID and a .ics extension.

### iCalendar File

A single iCalendar file can contain multiple events and is compatible with most calendar applications. This format is useful for simple import/export scenarios.

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

## GitHub Workflows

This project uses GitHub Actions for continuous integration:

- **Test and Build**: Ensures code quality by running tests and verifying that the code builds successfully across different Go versions
- **Release**: Automatically creates binaries for multiple platforms when a new tag is pushed

## Future Improvements

- Enhanced timezone handling for better cross-timezone event management
- Better support for complex recurring event patterns
- MIME format improvements for better compatibility with various email clients
- Calendar event filtering options
- Notifications and reminders integration

## License

Apache License 2.0