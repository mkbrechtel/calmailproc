# Test 10

Test 10 tests handling of emails with inline vcalendar content (not multipart). The calendar data is directly embedded in the email body rather than as an attachment or multipart structure.

## Expected state

Event: Test 10
Date: April 17, 2025
Time: 14:00 - 15:00 (W. Europe Standard Time)
Status: TENTATIVE (based on X-MICROSOFT-CDO-BUSYSTATUS)
Organizer: "Test Sender" (example@example.com)
Attendee: Markus Brechtel (markus.brechtel@uk-koeln.de)
Description: "empty"
Location: [Contains a Zoom meeting URL]
Reminder: 15 minutes before

Special characteristics: 
- Calendar data is inline in the email body, not multipart
- Includes timezone definition (W. Europe Standard Time)
- Has Microsoft-specific properties
- Contains a VALARM component with a 15-minute reminder

The calendar processor should correctly extract the event information from inline vcalendar content, rather than from attachments or multipart structure.