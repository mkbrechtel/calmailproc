# Test 15

Test 15 tests handling of invalid calendar data that has duplicate DTSTAMP properties.

## Expected behavior

This test email should be rejected with an error message. We expect an error message like this:

```
Error: error processing stdin: processing stdin: validation error for event oon6Angumod2xoj5Eashe7lee8xahvai: encoding calendar: encoding calendar: ical: failed to encode "VEVENT": want exactly one "DTSTAMP" property, got 2
```

The specific error occurs during validation, where the event has duplicate DTSTAMP properties when the iCalendar specification (RFC 5545) requires exactly one DTSTAMP property per event.

When processing this email, the calendar processor should:
1. Parse the iCalendar data successfully
2. Validate the event against the iCalendar specification
3. Detect the violation (duplicate DTSTAMP properties)
4. Report an appropriate error message
5. Continue processing other emails without crashing

This test ensures that the calendar processor correctly validates calendar data according to the iCalendar specification and rejects events with duplicate required properties.