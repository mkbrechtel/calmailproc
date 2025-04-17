# Test 12

Test 12 tests handling of invalid calendar data that should cause an error.

## Expected behavior

This test email should be rejected with an error message. We expect an error message like this:

```
Error: error processing stdin: processing stdin: parsing email: processing multipart: extracting calendar data: extracting calendar info: parsing iCal data: panic in decoder: runtime error: index out of range [0] with length 0
```

The specific error appears to be caused by a panic in the iCal decoder, indicating an index out of range error. This tests the processor's ability to gracefully handle malformed iCalendar data without crashing the entire application.

When processing this email, the calendar processor should:
1. Attempt to parse the iCalendar data
2. Encounter the malformed data
3. Report an appropriate error message
4. Continue processing other emails without crashing