# Title: Potential incomplete handling of calendar objects (VTIMEZONE and other components)

**Type:** Bug / Investigation Needed

**Status:** Open

## Description

The current implementation may not correctly handle all components of a calendar object when processing events. Specifically, when we extract and store VEVENT components, we might be losing important calendar metadata such as VTIMEZONE definitions and other non-VEVENT components.

## Potential Issues

1. **VTIMEZONE components**: Calendar invitations often include VTIMEZONE definitions that describe custom timezone rules. If we extract only the VEVENT component, we may lose this timezone information.

2. **Other calendar components**: The iCalendar format supports multiple component types:
   - VTIMEZONE: Timezone definitions
   - VALARM: Alarm/reminder definitions
   - VJOURNAL: Journal entries
   - VFREEBUSY: Free/busy time information
   - Other custom components

3. **Calendar properties**: Top-level calendar properties (PRODID, VERSION, CALSCALE, etc.) may not be preserved correctly.

## Current Behavior (Suspected)

When processing calendar data in `processor/processor.go`:
- We parse the calendar to extract VEVENT data
- We may be discarding VTIMEZONE and other components
- When merging recurring events or updating attendees, we might not preserve all calendar components
- Stored calendar files might be incomplete or invalid

## Areas to Review

1. **Event parsing** (`parser/ical/ical.go`):
   - How do we extract events from the calendar?
   - Do we preserve the full calendar context?

2. **Event storage** (`processor/processor.go`):
   - When we call `prepareEventForStorage()`, do we keep all calendar components?
   - When merging recurring events, do we preserve VTIMEZONE definitions?

3. **Calendar manipulation** (`handleRecurringEvent()`, `handleParentEventUpdate()`):
   - Do these functions preserve non-VEVENT components?
   - Are VTIMEZONE definitions carried over when merging calendars?

4. **Storage backends** (`storage/caldav.go`):
   - Do we store the complete calendar object or just the VEVENT?
   - Should we be storing VCALENDAR objects instead of just VEVENTs?

## Expected Behavior

Calendar objects should be stored completely with:
- All VTIMEZONE definitions referenced by events
- All VEVENT components (master events and instances)
- All calendar-level properties
- All related components (VALARM, etc.)

## Steps to Investigate

1. Review the `Event.RawData` field - does it contain the full VCALENDAR or just VEVENT?
2. Check test cases with timezone-specific events
3. Examine stored calendar files on CalDAV server
4. Test with an event that has:
   - Custom timezone definition
   - Multiple instances
   - Alarms/reminders
5. Compare input calendar object with stored output

## Proposed Solution (Tentative)

1. Ensure `Event.RawData` always contains the complete VCALENDAR object, not just VEVENT
2. When merging/updating calendars, preserve all VTIMEZONE and other components
3. Add validation to ensure required VTIMEZONE components are present
4. Add tests for timezone handling

## Test Cases Needed

1. Calendar with custom VTIMEZONE definition
2. Event with timezone reference (DTSTART;TZID=...)
3. Recurring event with timezone
4. Event with VALARM components
5. Multiple events sharing the same VTIMEZONE

## Additional Context

RFC 5545 (iCalendar) specifies that:
- VCALENDAR is the top-level component
- VTIMEZONE components define timezone rules
- Events with TZID parameters must reference a VTIMEZONE in the same calendar

Losing VTIMEZONE data could cause:
- Events to appear at wrong times
- Calendar clients to fail parsing
- Timezone conversion errors

## Priority

Medium-High - This could affect data integrity and event display times.
