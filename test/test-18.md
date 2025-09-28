# Test 18: Instance Update Before Parent Event

## Description
This test verifies correct handling when a recurring event instance update (with RECURRENCE-ID) is received before the parent recurring event.

## Scenario
1. **test-18-0.eml**: Instance update for October 3rd, 2025 (RECURRENCE-ID present)
   - Modifies specific instance: time changed from 10:00-11:00 to 11:00-12:00
   - SEQUENCE: 2
   - Has RECURRENCE-ID: 20251003T100000Z

2. **test-18-1.eml**: Parent recurring event (received after instance)
   - Weekly meeting starting September 26th, 2025
   - Every Friday, 10 occurrences total
   - Original time: 10:00-11:00
   - SEQUENCE: 0
   - Has RRULE but no RECURRENCE-ID

## Expected Behavior
The system should:
1. Process the instance update first (creating an exception)
2. Process the parent recurring event
3. Result in a recurring event series with one modified instance

## Related Tests
- Similar to test-17 which also tests instance-before-parent ordering