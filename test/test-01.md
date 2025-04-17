# Test 01

Test 01 is a simple calendar event creation followed by a cancellation.

The first mail creates a standard calendar event. The second mail cancels the same event with an increased sequence number.

## Expected state progression of the mail chain

## After mail 1
Event: Test Event 1
Date: March 10, 2025
Time: 14:00 - 15:00 (W. Europe Standard Time)
Status: CONFIRMED
Organizer: Markus Brechtel (markus.brechtel@uk-koeln.de)
Attendee: brechtel@med.uni-frankfurt.de

## After mail 2
Event: Cancelled: Test Event 1
Date: March 10, 2025
Time: 14:00 - 15:00 (W. Europe Standard Time)
Status: CANCELLED
Organizer: Markus Brechtel (markus.brechtel@uk-koeln.de)
Attendee: brechtel@med.uni-frankfurt.de

This event has been cancelled.

The second email has a sequence number (1) higher than the first email (0), indicating it supersedes the first message.