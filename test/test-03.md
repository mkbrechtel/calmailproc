# Test 03

Test 03 tests handling of a recurring event with specific instance modifications and cancellations.

The first mail creates a weekly recurring event. The second mail modifies a specific instance of the recurring event. The third mail cancels another specific instance of the recurring event.

## Expected state progression of the mail chain

## After mail 1
Event: Test 3
Date: March 13, 2025 (weekly on Thursdays until August 28, 2025)
Time: 15:00 - 16:00 (W. Europe Standard Time)
Status: CONFIRMED
Organizer: Markus Brechtel (markus.brechtel@uk-koeln.de)
Attendee: brechtel@med.uni-frankfurt.de

## After mail 2
The event instance on March 20, 2025 has been modified:

Event: Test 3
Date: March 20, 2025
Time: 11:00 - 12:00 (W. Europe Standard Time) - changed from 15:00-16:00
Status: CONFIRMED
Organizer: Markus Brechtel (markus.brechtel@uk-koeln.de)
Attendee: brechtel@med.uni-frankfurt.de

All other recurring instances remain at their original time.

## After mail 3
The event instance on March 27, 2025 has been cancelled:

Event: Cancelled: Test 3
Date: March 27, 2025
Time: 15:00 - 16:00 (W. Europe Standard Time)
Status: CANCELLED
Organizer: Markus Brechtel (markus.brechtel@uk-koeln.de)
Attendee: brechtel@med.uni-frankfurt.de

This specific instance has been cancelled.

All other recurring instances remain as scheduled, including the modified instance on March 20.

This test demonstrates how specific instances of a recurring event can be modified or cancelled without affecting the rest of the recurring series.