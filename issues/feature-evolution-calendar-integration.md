# Title: Add storage backend for GNOME/Evolution calendar integration

**Type:** Feature Request

**Status:** Open

## Description

Add a local storage backend that integrates directly with GNOME Evolution's calendar data storage. This would allow calmailproc to process calendar emails and store events directly into Evolution's local calendar files without requiring a CalDAV server.

## Motivation

- Many desktop users use Evolution as their primary calendar/email client on Linux
- Running a CalDAV server just for local calendar management adds unnecessary complexity
- Direct integration would provide a simpler setup for desktop users
- Evolution stores calendars locally in a standard format that should be accessible

## Expected Behavior

Users should be able to configure calmailproc to write directly to Evolution's calendar storage:

```bash
# Process emails and store in local Evolution calendar
cat email.eml | calmailproc -evolution-calendar "Personal"
```

Or via configuration file:
```yaml
storage:
  type: evolution
  calendar: "Personal"
```

## Technical Considerations

- Evolution stores calendar data in `~/.local/share/evolution/calendar/`
- Uses EDS (Evolution Data Server) which has its own storage format
- May need to interact with EDS via D-Bus or directly access the calendar files
- Need to handle file locking and concurrent access properly
- Should respect Evolution's calendar organization (multiple calendars, categories, etc.)

## Proposed Solution

Implement a new storage backend (`storage/evolution.go`) that:
1. Locates Evolution's calendar data directory (typically via XDG)
2. Interfaces with Evolution Data Server (EDS) via D-Bus or library bindings
3. Implements the standard Storage interface to create/update/delete events
4. Handles proper locking and synchronization with Evolution

Alternative approach:
- Directly read/write Evolution's calendar files if format is documented
- Monitor for changes to handle conflicts

## Additional Context

Related calendar systems that might use similar approaches:
- GNOME Calendar (also uses EDS)
- Other Evolution-compatible calendar applications

This feature would complement the existing CalDAV backend and provide a "local-first" option for desktop users.

## References

- Evolution Data Server documentation: https://wiki.gnome.org/Apps/Evolution
- EDS D-Bus interface documentation
- Calendar file format specifications
