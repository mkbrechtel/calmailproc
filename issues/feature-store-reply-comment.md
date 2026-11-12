# Title: Store the comment of an attendance reply

**Type:** Feature Request

**Status:** Open

## Description

`test/test-06.md` describes a decline with a comment ("no, sorry") and expects
the processor to "update the attendee status ... and store the comment without
changing any other event details".

calmailproc 0.2 updates the attendee's `PARTSTAT` but does not store the
comment. The reply carries it as a `COMMENT` property of its `VEVENT`:

```
ATTENDEE;PARTSTAT=DECLINED;CN=Markus Brechtel:MAILTO:markus.brechtel@uk-koeln.de
COMMENT;LANGUAGE=de-DE:no\, sorry\n
```

Version 0.1 appeared to store it, but only because a bug made it replace the
whole invitation with the reply.

## Expected Behavior

After processing the reply, the stored event shows the attendee as declined
and the comment is available with it.

## Actual Behavior

Only `PARTSTAT` is updated. The comment is dropped.

## Proposed Solution

Decide where the comment belongs in the stored event, since iCalendar has no
standard per-attendee comment. Options:

- the `X-RESPONSE-COMMENT` parameter on the `ATTENDEE` property, which several
  calendar clients display;
- a `COMMENT` property on the `VEVENT`, which loses who said it once two
  attendees comment.

Implement it in `update_attendee_status()` in `src/processor/mod.rs` and
assert it in `invitation_and_reply()` in `tests/processor.rs`.
