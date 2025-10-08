# Title: Add vdir storage backend

**Type:** Feature Request

**Status:** Open

## Description

Add a vdir storage backend to store calendar events in the vdir format. Vdir is a simple directory-based storage format where each calendar event is stored as an individual `.ics` file in a directory structure.

## Motivation

- Vdir is a simple, well-documented format used by popular tools like vdirsyncer and khal
- Provides a local, file-based storage option that doesn't require a server
- Enables easy integration with the vdirsyncer ecosystem for syncing calendars
- Simple to backup, inspect, and version control
- Works well with command-line calendar tools

## Expected Behavior

Users should be able to configure calmailproc to write events to a vdir:

```bash
# Process emails and store in vdir
cat email.eml | calmailproc -vdir ~/.calendars/personal
```

Or via configuration file:
```yaml
storage:
  type: vdir
  path: ~/.calendars/personal
```

## Technical Considerations

- Each event is stored as a separate `.ics` file named by UID
- File naming: `{UID}.ics`
- Directory structure is flat (single directory per calendar)
- Need to handle file system locking for concurrent access
- Should preserve all iCalendar properties
- Need to handle updates (overwrite existing files with same UID)
- Need to handle deletions

## Proposed Solution

Implement a new storage backend (`storage/vdir.go`) that:
1. Creates the vdir directory if it doesn't exist
2. Implements the Storage interface:
   - `StoreEvent()`: Write event to `{vdir_path}/{UID}.ics`
   - `GetEvent()`: Read event from `{vdir_path}/{UID}.ics`
   - `ListEvents()`: Read all `.ics` files in directory
   - `DeleteEvent()`: Remove `{vdir_path}/{UID}.ics`
3. Handles file locking appropriately
4. Validates UID for safe file naming

## Use Cases

- Local calendar storage without a CalDAV server
- Integration with khal (command-line calendar)
- Integration with vdirsyncer for multi-device sync
- Simple backup and version control of calendar data
- Debugging and inspection of calendar events

## Additional Context

Vdir format specification: https://vdirsyncer.pimutils.org/en/stable/vdir.html

Compatible tools:
- vdirsyncer: Synchronization tool for calendars and contacts
- khal: Command-line calendar application
- todoman: Command-line todo manager

This would complement the CalDAV backend by providing a simpler, local-first storage option.
