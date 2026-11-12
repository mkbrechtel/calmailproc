//! Deciding which of two versions of an event is newer.

use chrono::NaiveDateTime;

use super::{Event, decode};
use crate::error::{Context, Error, Result};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Comparison {
    FirstNewer,
    SecondNewer,
    Equal,
}

/// Compares two events: the higher SEQUENCE wins; with equal sequences the
/// later DTSTAMP wins. An event without a usable DTSTAMP loses.
pub fn compare_events(first: &Event, second: &Event) -> Result<Comparison> {
    if first.sequence > second.sequence {
        return Ok(Comparison::FirstNewer);
    }
    if first.sequence < second.sequence {
        return Ok(Comparison::SecondNewer);
    }

    match (
        extract_dtstamp(&first.raw_data),
        extract_dtstamp(&second.raw_data),
    ) {
        (Err(e1), Err(e2)) => Err(Error::new(format!(
            "failed to extract DTSTAMP from both events: {e1}, {e2}"
        ))),
        (Err(_), Ok(_)) => Ok(Comparison::SecondNewer),
        (Ok(_), Err(_)) => Ok(Comparison::FirstNewer),
        (Ok(a), Ok(b)) if a > b => Ok(Comparison::FirstNewer),
        (Ok(a), Ok(b)) if a < b => Ok(Comparison::SecondNewer),
        (Ok(_), Ok(_)) => Ok(Comparison::Equal),
    }
}

/// The DTSTAMP of the first VEVENT.
fn extract_dtstamp(ics: &str) -> Result<NaiveDateTime> {
    let calendar = decode(ics).context("parsing iCal data for DTSTAMP")?;
    let vevent = calendar
        .events()
        .next()
        .ok_or_else(|| Error::new("no VEVENT component found"))?;
    let dtstamp = vevent
        .value("DTSTAMP")
        .ok_or_else(|| Error::new("VEVENT missing DTSTAMP property"))?;
    parse_ical_time(dtstamp)
}

/// Parses an iCalendar timestamp such as `20250417T112140Z`.
///
/// A missing `Z` is treated as UTC. The non-standard `2023-07-01T122600Z` is
/// accepted too, because real mails contain it (test 16).
pub fn parse_ical_time(time: &str) -> Result<NaiveDateTime> {
    let time = time.strip_suffix('Z').unwrap_or(time);
    NaiveDateTime::parse_from_str(time, "%Y%m%dT%H%M%S")
        .or_else(|_| NaiveDateTime::parse_from_str(time, "%Y-%m-%dT%H%M%S"))
        .map_err(|e| Error::new(format!("parsing time {time:?}: {e}")))
}

#[cfg(test)]
mod tests {
    use super::*;
    use chrono::NaiveDate;

    const EARLY: &str = "20250417T110049Z";
    const LATE: &str = "20250417T112140Z";

    fn event(sequence: i64, dtstamp: Option<&str>) -> Event {
        let dtstamp = dtstamp
            .map(|d| format!("DTSTAMP:{d}\r\n"))
            .unwrap_or_default();
        Event {
            sequence,
            raw_data: format!(
                "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\n{dtstamp}DTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"
            ),
            ..Event::default()
        }
    }

    #[test]
    fn compare() {
        use Comparison::*;
        let cases = [
            (
                "higher sequence wins",
                event(2, Some(EARLY)),
                event(1, Some(LATE)),
                FirstNewer,
            ),
            (
                "lower sequence loses",
                event(0, Some(LATE)),
                event(1, Some(EARLY)),
                SecondNewer,
            ),
            (
                "same sequence, newer timestamp wins",
                event(0, Some(LATE)),
                event(0, Some(EARLY)),
                FirstNewer,
            ),
            (
                "same sequence, older timestamp loses",
                event(0, Some(EARLY)),
                event(0, Some(LATE)),
                SecondNewer,
            ),
            (
                "equal events",
                event(1, Some(EARLY)),
                event(1, Some(EARLY)),
                Equal,
            ),
            (
                "missing DTSTAMP in first event",
                event(1, None),
                event(1, Some(EARLY)),
                SecondNewer,
            ),
            (
                "missing DTSTAMP in second event",
                event(1, Some(EARLY)),
                event(1, None),
                FirstNewer,
            ),
        ];
        for (name, first, second, expected) in cases {
            assert_eq!(compare_events(&first, &second).unwrap(), expected, "{name}");
        }
    }

    #[test]
    fn compare_fails_without_any_dtstamp() {
        assert!(compare_events(&event(1, None), &event(1, None)).is_err());
    }

    #[test]
    fn parse_time() {
        let at = |y, mo, d, h, mi, s| {
            NaiveDate::from_ymd_opt(y, mo, d)
                .unwrap()
                .and_hms_opt(h, mi, s)
                .unwrap()
        };
        assert_eq!(
            parse_ical_time("20250417T112140Z").unwrap(),
            at(2025, 4, 17, 11, 21, 40)
        );
        assert_eq!(
            parse_ical_time("20250417T112140").unwrap(),
            at(2025, 4, 17, 11, 21, 40)
        );
        assert_eq!(
            parse_ical_time("2023-07-01T122600Z").unwrap(),
            at(2023, 7, 1, 12, 26, 0)
        );
        assert_eq!(
            parse_ical_time("2023-07-01T122600").unwrap(),
            at(2023, 7, 1, 12, 26, 0)
        );
        assert!(parse_ical_time("2025-04-17T11:21:40:123Z").is_err());
    }
}
