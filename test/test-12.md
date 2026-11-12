# Test 12

Test 12 tests handling of invalid calendar data that should cause an error.

## Expected behavior

This test email should be rejected with an error message. We expect an error message like this:

```
Error: error processing stdin: processing stdin: parsing email: extracting calendar data: in text/calendar part: parsing iCal data: decoding iCal data: malformed content line 8 without ':': ORGANIZER;CN=Sixt SE
```

The calendar attachment contains the content line `ORGANIZER;CN=Sixt SE`, which has no `:` and no value. The calendar is rejected before it is parsed. This tests the processor's ability to handle malformed iCalendar data gracefully.

When processing this email, the calendar processor should:
1. Attempt to parse the iCalendar data
2. Encounter the malformed data
3. Report an appropriate error message
4. Continue processing other emails without crashing