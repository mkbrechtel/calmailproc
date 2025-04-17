# Test 05

Test 05 tests handling of invitation and response workflow. The first email is an invitation, and the second email is an acceptance response.

## Expected state progression of the mail chain

## After mail 1
Event: Test 5
Date: March 19, 2025
Time: 10:00 - 11:00 (Europe/Berlin)
Status: CONFIRMED
Organizer: Markus Katharina Brechtel (markus.brechtel@thengo.net)
Attendees:
- markus.brechtel@thengo.net (NEEDS-ACTION)
- markus.brechtel@uk-koeln.de (NEEDS-ACTION)

## After mail 2
Event: Test 5
Date: March 19, 2025
Time: 10:00 - 11:00 (Europe/Berlin)
Status: CONFIRMED
Organizer: Markus Katharina Brechtel (markus.brechtel@thengo.net)
Attendees:
- markus.brechtel@thengo.net (NEEDS-ACTION)
- markus.brechtel@uk-koeln.de (ACCEPTED)

The second email is a response from one of the attendees accepting the invitation. The calendar processor should update the attendee status for markus.brechtel@uk-koeln.de from NEEDS-ACTION to ACCEPTED without changing any other event details.