We are trying to reproduce an issue with out of order messages in a complicated scenario with recurring event updates. This reproduces a real-world bug I encountered with a recurring event update when using the maildir processor.

Originally what is now the mail test-17-0.eml was test-17-8.eml. What happened here is that an update to the parent event is processed before the updates to recurring events exceptions. This leads to the event exceptions mails being ignored. 

Example output when this bug occurs:

```
Processing test/maildir/cur/test-17-0-8.eml with icalfile storage
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

