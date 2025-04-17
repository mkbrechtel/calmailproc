# Test 04

Test 04 tests handling of duplicate calendar event information. Both emails contain identical calendar data for the same event, sent to different recipients.

## Expected state progression of the mail chain

## After mail 1
Event: Test 4
Date: March 20, 2025
Time: 01:00 - 02:00 (UTC)
Status: CONFIRMED
Organizer: Markus Katharina Brechtel (brechtel@med.uni-frankfurt.de)
Attendees: 
- markus.brechtel@uk-koeln.de (NEEDS-ACTION)
- brechtel@med.uni-frankfurt.de (NEEDS-ACTION)

## After mail 2
No change to the event state. The calendar processor should recognize this as a duplicate event and maintain a single calendar entry.

Event: Test 4
Date: March 20, 2025
Time: 01:00 - 02:00 (UTC)
Status: CONFIRMED
Organizer: Markus Katharina Brechtel (brechtel@med.uni-frankfurt.de)
Attendees: 
- markus.brechtel@uk-koeln.de (NEEDS-ACTION)
- brechtel@med.uni-frankfurt.de (NEEDS-ACTION)

This test verifies that the calendar processor correctly handles duplicate events and doesn't create multiple entries when the same event information is sent to different recipients.