# Test 07

Test 07 tests handling of multiple sequential updates to an event. This series contains 5 emails showing progressive changes to the same event with increasing sequence numbers.

## Expected state progression of the mail chain

## After mail 1
Event: Test 7
Date: March 17, 2025
Time: 13:00 - 14:00 (UTC+01:00)
Status: CONFIRMED
Organizer: Markus Brechtel (markus.brechtel@uk-koeln.de)
Attendee: brechtel@med.uni-frankfurt.de
Sequence: 0

## After mail 2
Event: Test 7
Date: March 20, 2025
Time: 12:00 - 13:00 (UTC+01:00)
Status: CONFIRMED
Organizer: Markus Brechtel (markus.brechtel@uk-koeln.de)
Attendee: brechtel@med.uni-frankfurt.de
Sequence: 3

The date and time have been changed, and the sequence number has increased to 3.

## After mail 3-5
These emails contain additional updates to the same event, with increasing sequence numbers. Each update may modify different aspects of the event such as time, location, or attendees.

This test demonstrates how an event can evolve through multiple updates, and how the calendar processor should always apply the changes from the message with the highest sequence number.