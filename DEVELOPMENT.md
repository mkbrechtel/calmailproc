# CLAUDE.md - AI Assistant Guidelines

## Project: calmailproc
A calendar mail processor written in pure Go.

## Build Commands
- Build Go app: `go run main.go`
- Build the main app: `go build -o calmailproc ./cmd/calmailproc`
- Run tests: `go test ./...`
- Format code: `go fmt ./...`
- Run the app with stdin: `cat test/example-mail-1.eml | go run main.go`

## Style Guidelines
- **Formatting**: Follow Go standard formatting with `gofmt`
- **Naming**:
  - Use camelCase for unexported variables/functions
  - Use PascalCase for exported variables/functions
  - Use short, descriptive variable names
- **Error Handling**: Always check and handle errors explicitly
- **Comments**: Document public APIs with godoc style comments
- **Packages**: One package per directory, package name matches directory
- **Imports**: Group standard library, external, and internal imports
- **Types**: Use strong typing, avoid interface{} when possible

## Project Structure

### Modules

Separate modules for parsing mails and storing the calendar events in the database.

- **parser**: Responsible for parsing incoming emails to extract relevant information about calendar events.
- **storage**: Responsible for storing calendar events in the database. This module just implements a storage interface.
- **storage/vdir**: Implements a storage interface for storing calendar events in the [vdir format](https://vdirsyncer.readthedocs.io/en/stable/vdir.html)
- **processor**: Responsible for processing the parsed calendar events and storing them in the database.

### Event Storage

The calendar events come via mail as text/calendar attachments. We implement a storage interface for storing calendar events. We use the UID as the primary key for the events. The storage module provides methods for creating, updating, and deleting events.

### Testing 

The test mails are stored in the test/mail/ directory. We aim provide a test suite of realistic test mails to ensure the correctness of the parser and storage modules.

When testing the vdir storage module, we use the test/vdir directory. 

## Implementation Lessons
- Prefer standard library's `net/mail` package for basic email parsing rather than third-party libraries when possible
- For complex MIME parsing, the standard library has `mime` and `mime/multipart` packages
- Use `base64` package to decode encoded email attachments
- Writing tests first helps identify issues with parsing logic
- When creating CLI tools, support both human-readable and machine-readable (JSON) outputs
- Error handling should be graceful - if one part fails (like charset detection), continue with the rest
- Reading from stdin makes the tool composable with Unix pipelines

## Planned Improvements
- Better iCalendar parsing implementation
- Improved date/time parsing with timezone handling
- Support for recurring events
- Filtering capabilities

## License
Apache License 2.0
