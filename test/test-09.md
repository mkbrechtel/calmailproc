# Test 09

Test 09 tests handling of forwarded calendar invitations. This test contains a forwarded email with calendar data as attachments.

## Expected state

Event: Test 9
Date: April 16, 2025
Time: 04:00 - 05:00 (UTC)
Status: CONFIRMED
Organizer: Branson Lee (C.Lee@med.uni-frankfurt.de)
Attendee: Ludwig Montag (ludwig.montag@med.uni-frankfurt.de)

Special characteristics: 
- This is a forwarded email (FW: in subject)
- Contains the same calendar data twice as attachments
- Contains both plain text and HTML parts
- Includes an embedded image in the email

The calendar processor should correctly extract the event information from the attachments in the forwarded email and create a single event instance.