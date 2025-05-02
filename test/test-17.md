# Test 17: Recurring Event Instance Updates vs. Parent Updates

This test reproduces a real-world bug where recurring event instances are not properly processed when a higher-sequence parent event update is processed first.

## Problem Description

When processing calendar emails in an arbitrary order (as often happens with maildir), the following issue can occur:

1. A recurring event is initially created (sequence 0)
2. Several instance-specific updates are made for specific occurrences (sequences 0-6)
3. Later, an update to the parent/master event is made with a higher sequence number (sequence 7)
4. If the parent update (sequence 7) is processed BEFORE the instance updates, all instance updates are incorrectly ignored

The root issue is that when a master/parent event with a high sequence number (7) is processed first, the comparison logic treats all subsequent instance updates as "older events" because of their lower sequence numbers. However, instance updates should be processed regardless of the master event's sequence number, as they affect different occurrences.

## Current Output (Bug Example)

```
Processing test/maildir/cur/test-17-0.eml with icalfile storage
Stored new event with UID 040000008200E00074C5B7101A82E0080000000060FA38123DBBDB010000000000000000100000009123BEADE9978A4AA0AC92EF2005A108

Processing test/maildir/cur/test-17-1.eml with icalfile storage
Not processing older event (sequence: 0 vs 7, DTSTAMP comparison) with UID 040000008200E00074C5B7101A82E0080000000060FA38123DBBDB010000000000000000100000009123BEADE9978A4AA0AC92EF2005A108

Processing test/maildir/cur/test-17-2.eml with icalfile storage
Not processing older event (sequence: 2 vs 7, DTSTAMP comparison) with UID 040000008200E00074C5B7101A82E0080000000060FA38123DBBDB010000000000000000100000009123BEADE9978A4AA0AC92EF2005A108

Processing test/maildir/cur/test-17-3.eml with icalfile storage
Not processing older event (sequence: 1 vs 7, DTSTAMP comparison) with UID 040000008200E00074C5B7101A82E0080000000060FA38123DBBDB010000000000000000100000009123BEADE9978A4AA0AC92EF2005A108

Processing test/maildir/cur/test-17-4.eml with icalfile storage
Not processing older event (sequence: 3 vs 7, DTSTAMP comparison) with UID 040000008200E00074C5B7101A82E0080000000060FA38123DBBDB010000000000000000100000009123BEADE9978A4AA0AC92EF2005A108

Processing test/maildir/cur/test-17-5.eml with icalfile storage
Not processing older event (sequence: 4 vs 7, DTSTAMP comparison) with UID 040000008200E00074C5B7101A82E0080000000060FA38123DBBDB010000000000000000100000009123BEADE9978A4AA0AC92EF2005A108

Processing test/maildir/cur/test-17-6.eml with icalfile storage
Not processing older event (sequence: 6 vs 7, DTSTAMP comparison) with UID 040000008200E00074C5B7101A82E0080000000060FA38123DBBDB010000000000000000100000009123BEADE9978A4AA0AC92EF2005A108

Processing test/maildir/cur/test-17-7.eml with icalfile storage
Not processing older event (sequence: 5 vs 7, DTSTAMP comparison) with UID 040000008200E00074C5B7101A82E0080000000060FA38123DBBDB010000000000000000100000009123BEADE9978A4AA0AC92EF2005A108
```

## Expected Behavior

Instance-specific updates should be processed regardless of the parent event's sequence number. This requires modifications to the way recurring events are handled to ensure instance updates are properly preserved.

## Technical Details

- All emails have the same UID (`040000008200E00074C5B7101A82E0080000000060FA38123DBBDB010000000000000000100000009123BEADE9978A4AA0AC92EF2005A108`)
- test-17-0.eml: Parent event update with sequence 7
- test-17-1.eml through test-17-7.eml: Instance-specific updates with sequences 0-6
- The presence of `RECURRENCE-ID` in the instance updates indicates they modify specific occurrences, not the main event