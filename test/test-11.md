# Test 11

Test 11 is an out of sequence mail chain, where Outlook sends an cancelation but does not increase the sequence number.

The first mail is an event cancellation with a sequence number of 0. The second mail is an event request with a sequence number of 0.

This appears mostly in maildir mode, when the cancelation is parsed before the request.

## Expected state progression of the mail chain

## After mail 1
Event: Canceled: Test 11
Date: April 18, 2025
Time: 08:00 - 08:30 (W. Europe Standard Time)
Status: CANCELLED
Organizer: sender@example.org
Attendee: markus.brechtel@uk-koeln.de

This event has been cancelled.

## After mail 2
Event: Test 11
Date: April 18, 2025
Time: 08:00 - 08:30 (W. Europe Standard Time)
Status: CONFIRMED
Organizer: sender@example.org
Attendee: markus.brechtel@uk-koeln.de

Since both messages have the same sequence number (0), we need to check the DTSTAMP values to determine which change is more recent. The cancellation message has DTSTAMP 20250417T112140Z (11:21:40 UTC) while the request message has DTSTAMP 20250417T110049Z (11:00:49 UTC). The cancellation was sent after the request, making it the most recent action. Therefore, the event should remain in a cancelled state, despite the processing order in the maildir.
