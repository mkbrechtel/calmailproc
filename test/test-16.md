# Test 16

Test 16 tests handling of calendar events with non-standard formatted DTSTAMP values.

## Expected behavior

Test 16 consists of two emails:
- test-16-1.eml: Contains a valid calendar event with a DTSTAMP in a non-standard ISO format
- test-16-2.eml: Contains an update to the same event with the same non-standard ISO DTSTAMP format

Both emails should be processed successfully, with the system properly handling ISO-formatted date-time stamps (2023-07-01T122600Z) by falling back to ISO format parsing when the standard iCalendar format (20230701T122600Z) fails.

The first email (test-16-1.eml) should be processed successfully and the event should be stored, with the processor handling the non-standard DTSTAMP format.

The second email (test-16-2.eml) should also be processed successfully and treated as an update to the first event, even though they contain identical data.

When processing these emails, the calendar processor should:
1. Successfully process and store the first email
2. Successfully compare the second event with the stored first event
3. Properly parse the non-standard ISO DTSTAMP format
4. Process the second event as an update to the first 
5. Return a message indicating the event was updated

This test ensures that the calendar processor can handle and compare events with non-standard ISO time formats in addition to the standard iCalendar format, providing better interoperability with calendar systems that may use different timestamp formats.