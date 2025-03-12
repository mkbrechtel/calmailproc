# calmailproc - Development Guide

## Overview
calmailproc is a Go application that processes emails containing calendar data (iCalendar/ICS), extracts calendar events, and stores them in a structured format. It provides a clean, modular design with clear separation of concerns.

## Core Components and Responsibilities

### 1. Parser Module (`/parser`)
**Primary responsibility**: Extract calendar events from emails without making assumptions or modifications to the data.

- **Input**: Raw email content (RFC822 format)
- **Output**: Email metadata + calendar event data as found in the email
- **Constraints**:
  - Must not interpret or validate calendar semantics - extract only
  - Must preserve original calendar data for storage modules
  - Must handle common email encodings (base64, quoted-printable)
  - May extract metadata for identification (UID, summary, sequence number)

### 2. Storage Module (`/storage`)

**Primary responsibility**: Provide a consistent interface for storing and retrieving calendar events.

- **Interface**:
  - `StoreEvent(event *parser.CalendarEvent) error`
  - `GetEvent(id string) (*parser.CalendarEvent, error)`
  - `ListEvents() ([]*parser.CalendarEvent, error)`
  - `DeleteEvent(id string) error`

- **Key implementations**:
  - **Memory storage**: In-memory implementation for testing
  - **vdir storage**: File-based implementation using vdir format
  - **icalfile storage**: Single-file calendar implementation

- **Constraints**:
  - Must handle iCalendar format correctly without corrupting data
  - Must use UID as the primary identifier for events
  - Must check sequence numbers to avoid overwriting newer events with older ones
  - Must implement atomic operations where possible

### 3. Processor Module (`/processor`)

**Primary responsibility**: Orchestrate flow between parser and storage, applying business logic.

- **Core functions**:
  - Determine whether events should be stored or ignored
  - Handle event updates and sequence numbers
  - Apply business rules for METHOD attributes (REQUEST, CANCEL, REPLY)
  - Format and present output to the user

- **Constraints**:
  - Should not directly parse or modify calendar data
  - Should validate basic requirements before storage
  - Should handle errors gracefully

### 4. Main Application

**Primary responsibility**: Handle user input, configure components, and set up the processing pipeline.

- **Functions**:
  - Parse command-line arguments
  - Initialize appropriate storage backend
  - Process input source (stdin or maildir)
  - Output results in user-specified format

## Data Flow

1. Email is read from stdin or maildir
2. Parser extracts calendar data without modification
3. Processor applies business logic
4. Storage saves the event according to its implementation rules
5. Output is presented to the user

## Key Design Principles

1. **Clear separation of concerns**:
   - Parser: Extract data only, don't interpret
   - Storage: Store/retrieve data, maintain data integrity
   - Processor: Apply business logic, make decisions

2. **Error handling**:
   - Each layer should handle its own errors
   - Don't throw exceptions or panic (except in catastrophic cases)
   - Return errors up the call stack with context
   - Fail gracefully where possible

3. **Immutable data flow**:
   - Parser should produce data without modification
   - Processor should make decisions but not modify raw data
   - Storage should handle all format-specific transformations

4. **Simplicity over complexity**:
   - Prefer clear, simple code over clever optimizations
   - Each function should do one thing well
   - Avoid special cases and excessive error handling
   - Use the standard library where possible

## Improvement Recommendations

To rebuild the project with a cleaner architecture:

1. **Parser module refactoring**:
   - Create a clean extraction layer that doesn't attempt calendar validation
   - Keep extracted data minimal - only ID, sequence, and method are needed for decisions
   - Preserve raw bytes for storage layer

2. **Storage layer refinement**:
   - Move all iCalendar-specific logic into storage implementations
   - Implement proper atomicity for file operations
   - Add validation of stored output

3. **Processor simplification**:
   - Implement clear decision trees with minimal branching
   - Move format-specific logic to appropriate layers
   - Make business rules explicit and configurable

## Testing Strategy

1. **Unit tests**:
   - Parser: Test extraction from various email formats
   - Storage: Test CRUD operations on calendar data
   - Processor: Test business logic and decision making

2. **Integration tests**:
   - Test parser + processor + storage with realistic emails
   - Validate end-to-end flow maintains data integrity

3. **Test data**:
   - Maintain clean, realistic test emails in the test directory
   - Document the purpose of each test email
   - Include edge cases and error conditions

## Development Workflow

1. Run tests: `go test ./...`
2. Build: `go build`
3. Format code: `go fmt ./...`
4. Check for race conditions: `go test -race ./...`
5. Benchmark: `go test -bench=. ./...`