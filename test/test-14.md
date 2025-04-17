# Test 14

Test 14 contains an auto-generated iCalendar event from Trenitalia with a validation error.

## Issue Details

The iCalendar attachment is missing a required PRODID property in the VCALENDAR component. According to the iCalendar specification (RFC 5545), the PRODID property is mandatory for all VCALENDAR objects.

## Expected Behavior

The processor should:
1. Identify the missing PRODID property
2. Generate a user-friendly error message
3. Skip processing this event while continuing to process other valid events

## Error Message

```
validation error for event e516a376-6363-41d4-a373-23e021395068: encoding calendar: encoding calendar: ical: failed to encode "VCALENDAR": want exactly one "PRODID" property, got 0
```

## Original Event Data

- UID: e516a376-6363-41d4-a373-23e021395068
- SUMMARY: Reise Bologna Centrale-Ravenna, Zug Regionale TTPER 4001
- DTSTART: 20230929T165000
- DTEND: 20230929T175200
- LOCATION: Bologna Centrale
- ORGANIZER: webmaster@trenitalia.it

This test case demonstrates proper handling of malformed iCalendar data from real-world sources.