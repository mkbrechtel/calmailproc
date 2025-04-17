# Test 02

Test 02 is a calendar event update that changes the date and time of the event.

The first mail creates a standard calendar event. The second mail updates the same event with a different date and time, using an increased sequence number.

## Expected state progression of the mail chain

## After mail 1
Event: Test 2
Date: March 11, 2025
Time: 14:00 - 15:00 (W. Europe Standard Time)
Status: CONFIRMED
Organizer: Markus Brechtel (markus.brechtel@uk-koeln.de)
Attendee: brechtel@med.uni-frankfurt.de

## After mail 2
Event: Test 2
Date: March 12, 2025
Time: 16:00 - 17:00 (W. Europe Standard Time)
Status: CONFIRMED
Organizer: Markus Brechtel (markus.brechtel@uk-koeln.de)
Attendee: brechtel@med.uni-frankfurt.de

The second email has a sequence number (1) higher than the first email (0), indicating it supersedes the first message. The key change is the rescheduling of the event to a different date and time.