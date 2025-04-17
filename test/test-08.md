# Test 08

Test 08 tests handling of calendar events from non-Outlook systems. This test uses an event created in Horde webmail system.

## Expected state

Event: Test 8
Date: April 16, 2025
Time: 13:00 - 14:00 (UTC)
Status: CONFIRMED
Organizer: Branson Lee (C.Lee@med.uni-frankfurt.de)
Attendees: 
- Markus Brechtel (brechtel@med.uni-frankfurt.de)
- markus.brechtel@uk-koeln.de
Description: "Only Test not real Termin"

Special characteristics: The email contains the calendar data twice - once inline in text/calendar format and once as a base64-encoded attachment. The calendar processor should handle this correctly and create only one event instance.