# calmailproc - Development Guide

## Overview
calmailproc is a Go application that processes emails containing calendar data (iCalendar/ICS), extracts calendar events, and stores them in a structured format. It provides a clean, modular design with clear separation of concerns.

## Core Components and Responsibilities

### 1. Parser Modules 

The parser functionality is split into two modules with distinct responsibilities:

#### 1.1 Email Parser (`/parser/email`)
**Primary responsibility**: Parse email content without calendar-specific interpretation.

- **Input**: Raw email content (RFC822 format)
- **Output**: Email metadata with calendar event data if present
- **Constraints**:
  - Must extract email fields (subject, from, to, date)
  - Must detect presence of calendar attachments
  - Must handle common email encodings (base64, quoted-printable)
  - Must delegate calendar parsing to the ical module

#### 1.2 iCalendar Parser (`/parser/ical`)
**Primary responsibility**: Parse and provide access to calendar data.

- **Input**: Raw iCalendar data (from email attachments)
- **Output**: Structured calendar event data
- **Constraints**:
  - Must extract essential calendar fields (UID, summary, sequence number)
  - Must preserve original calendar data for storage modules
  - Must centralize all iCalendar parsing functionality
  - May extract additional metadata for identification

### 2. Storage Module (`/storage`)

**Primary responsibility**: Provide a consistent interface for storing and retrieving calendar events.

- **Interface**:
  - `StoreEvent(event *ical.Event) error`
  - `GetEvent(id string) (*ical.Event, error)`
  - `ListEvents() ([]*ical.Event, error)`
  - `DeleteEvent(id string) error`

- **Key implementations**:
  - **Memory storage** (`'/storage/memory` module): In-memory implementation for testing
  - **vdir storage** (`'/storage/vdir` module): File-based implementation using vdir format
  - **icalfile storage** (`'/storage/icalfile` module): Single-file calendar implementation

- **Constraints**:
  - Must handle iCalendar format correctly without corrupting data
  - Must use UID as the primary identifier for events
  - Must check sequence numbers to avoid overwriting newer events with older ones
  - Must implement atomic operations where possible

### 3. Calendar Manager Module (`/manager`)

**Primary responsibility**: Handle all calendar event manipulations and business logic.

- **Core functions**:
  - Update attendee status based on REPLY methods
  - Handle recurring event updates (instances with RECURRENCE-ID)
  - Provide centralized calendar manipulation functionality
  - Ensure calendar data integrity

- **Constraints**:
  - Should maintain clean separation from storage logic
  - Should encapsulate all calendar-specific manipulations
  - Should focus on logical operations, not persistence

### 4. Processor Module (`/processor`)

**Primary responsibility**: Orchestrate flow between parser, manager, and storage, applying business logic.

- **Core functions**:
  - Determine whether events should be stored or ignored
  - Handle event updates and sequence numbers
  - Apply business rules for METHOD attributes (REQUEST, CANCEL, REPLY)
  - Format and present output to the user

- **Constraints**:
  - Should not directly parse or modify calendar data
  - Should validate basic requirements before storage
  - Should handle errors gracefully

### Processor Sub-modules

The processor module is divided into specialized sub-modules for each operation mode:

#### 4.1 Shared Processor functionality (`/processor`)

**Primary responsibility**: Handle general event processing logic common to all modes.

- **Core functions**:
  - Implement sequence number checking
  - Apply METHOD handling rules
  - Provide decision framework for event storage
  - Coordinate between the Calendar Manager and Storage

#### 4.2 Stdin Processor (`/processor/stdin`)

**Primary responsibility**: Process single emails from stdin with immediate feedback.

- **Core functions**:
  - Stream processing with minimal memory usage
  - Immediate output formatting
  - Return appropriate exit codes for pipeline integration

#### 4.3 Maildir Processor (`/processor/maildir`)

**Primary responsibility**: Batch process multiple emails from maildir structure.

- **Core functions**:
  - Efficiently iterate through maildir hierarchies
  - Track processed emails to avoid duplicates
  - Generate batch summary statistics
  - Handle directory locking and file status transitions

### 5. Main Application

**Primary responsibility**: Handle user input, configure components, and set up the processing pipeline.

- **Functions**:
  - Parse command-line arguments
  - Initialize appropriate storage backend
  - Process input source (stdin or maildir)
  - Output results in user-specified format

## Data Flow

1. Email is read from stdin or maildir
2. Email parser extracts basic email data and identifies calendar attachments
3. iCalendar parser extracts calendar event data when present
4. Processor applies general business logic to the parsed data
5. Calendar Manager handles any calendar-specific operations (attendee updates, recurring event handling)
6. Storage saves the event according to its implementation rules
7. Output is presented to the user

## Key Design Principles

1. **Clear separation of concerns**:
   - Email Parser: Extract email data only
   - iCalendar Parser: Parse calendar data only
   - Calendar Manager: Handle calendar-specific logic and manipulations
   - Storage: Store/retrieve data, maintain data integrity
   - Processor: Orchestrate flow, make high-level decisions

2. **Error handling**:
   - Each layer should handle its own errors
   - Don't throw exceptions or panic (except in catastrophic cases)
   - Return errors up the call stack with context
   - Fail gracefully where possible

3. **Immutable data flow**:
   - Email parser should produce email data without interpretation
   - iCalendar parser should extract calendar data without modifying it
   - Calendar Manager should handle calendar-specific manipulations
   - Processor should coordinate but not directly modify data
   - Storage should handle all persistence-related transformations

4. **Simplicity over complexity**:
   - Prefer clear, simple code over clever optimizations
   - Each function should do one thing well
   - Avoid special cases and excessive error handling
   - Use the standard library where possible

## Improvement Recommendations

To further improve the project architecture:

1. **Parser modules refinement**:
   - ✅ Split parsing into email and calendar components - DONE
   - ✅ Create a clean extraction layer that doesn't attempt calendar validation - DONE
   - ✅ Keep extracted data minimal - only ID, sequence, and method are needed for decisions
   - ✅ Preserve raw bytes for storage layer - DONE

2. **Storage layer refinement**:
   - ✅ Use the ical.Event type consistently across all storage implementations - DONE
   - ✅ Use the parser/ical module for all calendar operations - DONE
   - Implement proper atomicity for file operations
   - Add validation of stored output

3. **Processor simplification**:
   - Implement clear decision trees with minimal branching
   - Move format-specific logic to appropriate layers
   - Make business rules explicit and configurable

4. **✅ Calendar Manager module**:
   - ✅ Implement calendar manipulation logic in separate module - DONE
   - ✅ Extract attendee update logic from storage modules - DONE
   - ✅ Extract recurring event handling from storage modules - DONE
   - Implement future calendar-specific operations in this module

## Calendar Event Handling

The application handles various types of calendar events according to the iCalendar specification (RFC 5545). Here's how each type should be handled:

### Event Types and Methods

1. **REQUEST (New Event or Update)**
   - Email Parser: Detect email with calendar attachment
   - iCal Parser: Extract UID, sequence number, and raw data
   - Processor: Check for existing event with same UID
   - Storage: Store as new event if UID not found
   - Storage: Update existing event if sequence number is higher

2. **CANCEL (Event Cancellation)**
   - Email Parser: Detect email with calendar attachment
   - iCal Parser: Extract UID, sequence number, and method (CANCEL)
   - Processor: Check for existing event with same UID
   - Storage: Update event status to CANCELLED
   - Storage: Do not overwrite if existing sequence number is higher

3. **REPLY (Attendance Response)**
   - Email Parser: Detect email with calendar attachment
   - iCal Parser: Extract UID, method (REPLY), and raw data
   - Processor: Check if reply processing is enabled
   - Processor: If enabled, retrieve existing event and pass to Calendar Manager
   - Calendar Manager: Update participant status in event
   - Storage: Store the updated event

### Recurring Events

Recurring events require special handling:

1. **Master Recurring Event**
   - Has RRULE property but no RECURRENCE-ID
   - Storage: Store as normal event

2. **Exception to Recurring Event**
   - Has both RRULE and RECURRENCE-ID
   - Processor: Identify as modification to specific instance
   - Calendar Manager: Handle recurring event update
   - Storage: Store the updated calendar with exception

3. **Cancellation of Specific Instance**
   - Has RECURRENCE-ID and METHOD:CANCEL
   - Calendar Manager: Mark specific instance as cancelled without affecting master event
   - Storage: Store the updated calendar

### Sequence Numbers

Sequence numbers prevent out-of-order processing:

1. **Higher Sequence Number**
   - Newer version of an event
   - Should replace older versions

2. **Lower Sequence Number**
   - Older version of an event
   - Should be ignored if newer version exists

3. **Equal Sequence Number**
   - Special case for compatibility
   - May apply as update if implementation allows

## Testing Strategy

1. **Unit tests**:
   - Email Parser: Test extraction from various email formats
   - iCal Parser: Test calendar data extraction and handling
   - Calendar Manager: Test calendar manipulation functionality
   - Storage: Test CRUD operations on calendar data
   - Processor: Test orchestration and decision making

2. **Integration tests**:
   - Test parser + processor + storage with realistic emails
   - Validate end-to-end flow maintains data integrity

3. **Test data**:
   - Maintain clean, realistic test emails in the test directory
   - Document the purpose of each test email
   - Include edge cases and error conditions

## Operation Modes

The application operates in two distinct modes, each with its own workflow and use cases:

### 1. Stdin Mode

In Stdin mode, the application processes a single email message from standard input.

**Workflow:**
1. Email data is read from stdin
2. The parser extracts calendar information 
3. The processor applies business logic
4. If enabled, the event is stored in the configured storage backend
5. Output is presented on stdout in the specified format (plain text or JSON)

**Use Cases:**
- Integration with mail delivery agents (e.g., procmail, sieve)
- Processing a single email via pipe in shell
- Testing and debugging individual email handling

**Example:**
```bash
cat test/maildir/cur/example-mail-01.eml | ./calmailproc --store
```

**Configuration Options:**
- `--store`: Enable storing events (default: false)
- `--format=json`: Output in JSON format (default: plain text)
- `--process-replies`: Process METHOD:REPLY emails (default: false)

### 2. Maildir Mode

In Maildir mode, the application processes multiple emails from a maildir directory structure.

**Workflow:**
1. Scan the maildir directory for email files
2. Process each email file through the parser and processor
3. Store events in the configured backend (if enabled)
4. Generate a summary of processed emails
5. Optionally scan for subdirectories and process them recursively

**Maildir Structure:**
- `new/`: Directory for new, unread mail
- `cur/`: Directory for read mail
- `tmp/`: Directory for temporary mail files

**Use Cases:**
- Batch processing of existing email archives
- Scheduled processing via cron jobs
- Initial migration of calendar data from email to calendar storage

**Example:**
```bash
./calmailproc --maildir=/var/mail/user/Calendars --store --recursive
```

**Configuration Options:**
- `--maildir`: Path to maildir directory
- `--recursive`: Process subdirectories recursively
- `--store`: Enable storing events
- `--process-replies`: Process METHOD:REPLY emails
- `--verbose`: Show detailed output for each email

## Error Handling Strategy

Error handling is crucial for a reliable application. Follow these principles:

### Error Categories

1. **Fatal Errors**: Application cannot continue
   - Configuration errors
   - Critical resource unavailability (e.g., storage not writable)
   - Exit with non-zero code and clear error message

2. **Operational Errors**: Can continue despite issues
   - Individual email parsing failures
   - Failed updates due to sequence rules
   - Log error and continue processing other items

3. **Data Validation Errors**: Input data problems
   - Malformed email format
   - Missing required calendar fields
   - Log issue, skip problematic entry, continue processing

### Error Handling Patterns

1. **Return errors with context**
   ```go
   if err != nil {
       return fmt.Errorf("parsing email: %w", err)
   }
   ```

2. **Log and continue**
   ```go
   events, err := store.ListEvents()
   if err != nil {
       log.Printf("Error listing events: %v, continuing with empty list", err)
       events = []*CalendarEvent{}
   }
   ```

3. **Graceful degradation**
   ```go
   // If we can't parse all calendar details, extract what we can
   if err := parseDetails(data); err != nil {
       log.Printf("Warning: Partial calendar data extracted: %v", err)
       // Continue with partial data
   }
   ```

### Avoiding Error Handling Pitfalls

1. **No Panics**: Never use panic except in truly unrecoverable scenarios
2. **No Silent Failures**: Always log errors, even if continuing
3. **No Special Error Types**: Use standard error wrapping/unwrapping
4. **Clear Error Messages**: Make error messages actionable and clear
5. **Consistent Return Types**: Don't return nil objects with side effects

## Development Workflow

1. Run tests: `go test ./...`
2. Build: `go build`
3. Format code: `go fmt ./...`
4. Check for race conditions: `go test -race ./...`
5. Benchmark: `go test -bench=. ./...`
6. End-to-end testing: `./test.sh`

## Important notes for AI agents

Do not modify the end-to-end testing script (`./test.sh`). Only human developers should modify it.
