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
- `cmd/calmailproc/` - Main application entry point
- `internal/parser/` - Email and calendar parsing logic
- `test/` - Test data, example emails

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
