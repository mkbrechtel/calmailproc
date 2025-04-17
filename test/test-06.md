# Test 06

Test 06 tests handling of invitation and decline response workflow. The first email is an invitation, and the second email is a decline response with a comment.

## Expected state progression of the mail chain

## After mail 1
Event: Test 6
Date: March 18, 2025
Time: 09:15 - 10:15 (Europe/Berlin)
Status: CONFIRMED
Organizer: Markus Katharina Brechtel (markus.brechtel@thengo.net)
Attendees:
- Markus Katharina Brechtel (markus.brechtel@thengo.net)
- markus.brechtel@uk-koeln.de (NEEDS-ACTION)

## After mail 2
Event: Test 6
Date: March 18, 2025
Time: 09:15 - 10:15 (Europe/Berlin)
Status: CONFIRMED
Organizer: Markus Katharina Brechtel (markus.brechtel@thengo.net)
Attendees:
- Markus Katharina Brechtel (markus.brechtel@thengo.net)
- markus.brechtel@uk-koeln.de (DECLINED, Comment: "no, sorry")

The second email is a response from one of the attendees declining the invitation with a comment. The calendar processor should update the attendee status for markus.brechtel@uk-koeln.de from NEEDS-ACTION to DECLINED and store the comment without changing any other event details.