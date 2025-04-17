# Test 16

Test 16 tests handling of calendar events with incorrectly formatted DTSTAMP values.

## Expected behavior

Test 16 consists of two emails:
- test-16-1.eml: Contains a valid calendar event with a DTSTAMP in a non-standard format
- test-16-2.eml: Contains an update to the same event with the same non-standard DTSTAMP format

The first email (test-16-1.eml) should be processed successfully and the event should be stored, as the processor should be able to handle this specific DTSTAMP format variation.

The second email (test-16-2.eml) should be rejected with an error message when compared to the first event. We expect an error message like this:

```
Error: comparing events: failed to extract DTSTAMP from both events: parsing time "2023-07-01T122600Z" as "20060102T150405Z": cannot parse "-07-01T122600Z" as "01"
```

The specific error occurs during event comparison where the non-standard DTSTAMP format prevents proper comparison between events.

When processing these emails, the calendar processor should:
1. Successfully process and store the first email
2. Attempt to compare the second event with the stored first event
3. Encounter an error when parsing the non-standard DTSTAMP format during comparison
4. Report an appropriate error message
5. Continue processing other emails without crashing

This test ensures that the calendar processor can store events with slightly non-standard time formats, but identifies issues when comparing such events later on.