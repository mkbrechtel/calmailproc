//! The processor: decides what happens to the calendar events in an email.
//!
//! It orchestrates the email parser, the iCalendar module and storage, and
//! holds all business logic.
//!
//! A mail's calendar is split by UID; each UID is one stored event. Within an
//! event every component is handled on its own: the parent event, and each
//! update to a single occurrence (a VEVENT with RECURRENCE-ID). A component
//! replaces the stored one unless the stored one is newer, so the result does
//! not depend on the order in which mails are processed.

pub mod maildir;
pub mod stdin;

use chrono::{DateTime, Utc};
use serde::Deserialize;

use crate::email;
use crate::error::{Context, Error, Result};
use crate::ical::{self, Calendar, Comparison, Component, Event, Property};
use crate::storage::{Storage, StoreError, Stored};

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct ProcessorConfig {
    /// Apply attendance replies (METHOD:REPLY) to stored events.
    pub process_replies: bool,
    /// Store a reply even if its event is not stored.
    pub force: bool,
    /// Ignore a cancellation if its event is not stored. By default it is
    /// stored, so that the event stays cancelled when the request for it is
    /// processed later (test 11).
    pub ignore_unknown_cancellations: bool,
    /// Delete cancelled events and occurrences instead of marking them.
    pub delete_cancelled: bool,
    /// Remove alarms (VALARM) from events before storing them.
    pub drop_alarms: bool,
}

/// A failed processing step: what to tell the user, and the technical cause.
#[derive(Debug, Clone)]
pub struct Failure {
    pub status: String,
    pub error: Error,
}

/// The status message describing what was done, or a [`Failure`].
pub type Outcome = std::result::Result<String, Failure>;

pub const NO_CALENDAR_EVENT: &str = "Processed E-Mail without calendar event";

/// How often an event is read and merged again when someone else changed it
/// between our read and our write.
const ATTEMPTS: usize = 3;

pub struct Processor<S: Storage> {
    pub storage: S,
    pub config: ProcessorConfig,
}

/// What to do with a stored event after looking at a mail.
enum Action {
    Nothing(String),
    Store(Calendar, String),
    Delete(String),
}

impl<S: Storage> Processor<S> {
    pub fn new(storage: S, process_replies: bool) -> Self {
        Self::from_config(
            storage,
            &ProcessorConfig {
                process_replies,
                ..ProcessorConfig::default()
            },
        )
    }

    pub fn from_config(storage: S, config: &ProcessorConfig) -> Self {
        Processor {
            storage,
            config: config.clone(),
        }
    }

    /// Processes one raw email and returns a status message.
    pub fn process_email(&mut self, raw: &[u8]) -> Outcome {
        let parsed = email::parse(raw)
            .context("parsing email")
            .map_err(|e| fail("E-Mail parsing error", e))?;
        let Some(event) = parsed.event else {
            return Ok(NO_CALENDAR_EVENT.to_string());
        };
        let calendar = ical::decode(&event.raw_data)
            .context("parsing email")
            .map_err(|e| fail("E-Mail parsing error", e))?;
        let received = parsed
            .date
            .as_deref()
            .and_then(|date| DateTime::parse_from_rfc3339(date).ok())
            .map(|date| date.with_timezone(&Utc))
            .unwrap_or_else(Utc::now);

        // Every UID is an event of its own. All are processed; the first
        // failure is reported.
        let mut statuses = Vec::new();
        let mut failure = None;
        for (uid, events) in split_by_uid(&calendar).map_err(|e| fail("E-Mail parsing error", e))? {
            match self.process_calendar(&uid, &event.method, events, received) {
                Ok(status) => statuses.push(status),
                Err(error) => failure = failure.or(Some(error)),
            }
        }
        match failure {
            Some(failure) => Err(failure),
            None => Ok(statuses.join("; ")),
        }
    }

    /// Processes the components of one event.
    fn process_calendar(
        &mut self,
        uid: &str,
        method: &str,
        mut incoming: Calendar,
        received: DateTime<Utc>,
    ) -> Outcome {
        ical::validate_uid(uid)
            .map_err(|e| fail(format!("Invalid UID for calendar event: {e}"), e))?;
        if method == "REPLY" && !self.config.process_replies {
            return Ok("Ignoring calendar REPLY method as configured".to_string());
        }

        normalize(&mut incoming, received, &self.config);
        ical::validate(&incoming)
            .context("encoding calendar")
            .context(&format!("validation error for event {uid}"))
            .map_err(|e| fail(format!("Invalid calendar data for event with UID {uid}"), e))?;

        for _ in 0..ATTEMPTS {
            let stored = self
                .storage
                .get_event(uid)
                .context("looking up existing event")
                .map_err(|e| fail("Error looking up existing event", e))?;
            let stored_calendar = stored
                .as_ref()
                .map(|stored| ical::decode(&stored.event.raw_data))
                .transpose()
                .context("parsing existing event data")
                .map_err(|e| fail("Error reading stored event", e))?;

            let action = if method == "REPLY" {
                self.apply_reply(uid, &incoming, stored_calendar.as_ref())?
            } else {
                self.apply_update(uid, method, &incoming, stored_calendar.as_ref())?
            };
            match self.perform(uid, action, stored.as_ref(), stored_calendar.as_ref())? {
                Some(status) => return Ok(status),
                // Someone else changed the event in between: read it again.
                None => continue,
            }
        }
        Err(fail(
            "Error storing event",
            Error::new(format!(
                "storing event {uid}: the stored event was changed by someone else {ATTEMPTS} times in a row"
            )),
        ))
    }

    /// Carries out an action. Returns `None` if storage reported a conflict.
    fn perform(
        &mut self,
        uid: &str,
        action: Action,
        stored: Option<&Stored>,
        stored_calendar: Option<&Calendar>,
    ) -> std::result::Result<Option<String>, Failure> {
        let version = stored.map(|stored| &stored.version);
        let (result, status, failure_status) = match action {
            Action::Nothing(status) => return Ok(Some(status)),
            Action::Delete(status) => {
                let version = version.expect("only a stored event is deleted");
                (
                    self.storage.delete_event(version),
                    status,
                    "Error deleting event",
                )
            }
            Action::Store(calendar, status) => {
                let event = prepare_event_for_storage(uid, calendar, stored_calendar)
                    .context("preparing event")
                    .map_err(|e| fail("Error preparing event for storage", e))?;
                let failure_status = if stored.is_some() {
                    "Error storing updated event"
                } else {
                    "Error storing new event"
                };
                (
                    self.storage.store_event(&event, version),
                    status,
                    failure_status,
                )
            }
        };
        match result {
            Ok(()) => Ok(Some(status)),
            Err(StoreError::Conflict) => Ok(None),
            Err(StoreError::Other(error)) => Err(fail(failure_status, error)),
        }
    }

    /// REQUEST, CANCEL, PUBLISH and anything else that carries event data:
    /// merges the mail's components into the stored event.
    fn apply_update(
        &self,
        uid: &str,
        method: &str,
        incoming: &Calendar,
        stored: Option<&Calendar>,
    ) -> std::result::Result<Action, Failure> {
        let is_cancel = method == "CANCEL";
        let delete = is_cancel && self.config.delete_cancelled;

        let Some(stored) = stored else {
            // A cancellation for an event that is not stored is kept as a
            // cancelled event, unless configured otherwise: mails are not
            // always processed in the order they were sent, and the request
            // for it must not bring the event back (test 11).
            if is_cancel && (delete || self.config.ignore_unknown_cancellations) {
                return Ok(Action::Nothing(format!(
                    "Ignoring cancellation for unknown event with UID {uid}"
                )));
            }
            let mut new = incoming.clone();
            if is_cancel {
                new.events_mut().for_each(mark_cancelled);
            }
            return Ok(Action::Store(
                new,
                format!("Stored new event with UID {uid}"),
            ));
        };

        let had_occurrences = stored.events().any(is_occurrence);
        let mut merged = stored.clone();
        let mut new_parent_sequence = None;
        let mut occurrences_changed = 0;
        let mut occurrences_removed = 0;
        let mut delete_event = false;
        let mut skipped = None;

        for component in incoming.events() {
            let recurrence_id = component.value("RECURRENCE-ID");
            let position = merged
                .components
                .iter()
                .position(|c| c.name == "VEVENT" && c.value("RECURRENCE-ID") == recurrence_id);

            // An older version never replaces a newer one.
            if let Some(index) = position {
                let stored_component = &merged.components[index];
                let comparison = ical::compare_components(component, stored_component)
                    .context("comparing events")
                    .map_err(|e| fail("Error comparing events", e))?;
                if comparison == Comparison::SecondNewer {
                    skipped.get_or_insert((
                        ical::sequence(component),
                        ical::sequence(stored_component),
                    ));
                    continue;
                }
            }

            let mut component = component.clone();
            if is_cancel {
                mark_cancelled(&mut component);
            }
            match (recurrence_id.is_some(), delete) {
                // The parent event is cancelled and cancelled events are deleted.
                (false, true) => delete_event = true,
                // A new version of the parent event. Calendar properties and
                // time zones come with it; stored occurrence updates stay.
                (false, false) => {
                    new_parent_sequence = Some(ical::sequence(&component));
                    let mut components: Vec<Component> = incoming
                        .components
                        .iter()
                        .filter(|c| c.name != "VEVENT")
                        .cloned()
                        .collect();
                    components.push(component);
                    components.extend(
                        merged
                            .components
                            .iter()
                            .filter(|c| is_occurrence(c))
                            .cloned(),
                    );
                    let previous = std::mem::replace(
                        &mut merged,
                        Calendar {
                            name: incoming.name.clone(),
                            properties: incoming.properties.clone(),
                            components,
                        },
                    );
                    add_missing_time_zones(&mut merged, &previous);
                }
                // One occurrence is cancelled and cancelled ones are deleted.
                (true, true) => {
                    if let Some(index) = position {
                        merged.components.remove(index);
                    }
                    exclude_occurrence(&mut merged, &component);
                    occurrences_removed += 1;
                }
                // A new version of one occurrence.
                (true, false) => {
                    match position {
                        Some(index) => merged.components[index] = component,
                        None => merged.components.push(component),
                    }
                    add_missing_time_zones(&mut merged, incoming);
                    occurrences_changed += 1;
                }
            }
        }

        if delete_event {
            return Ok(Action::Delete(format!(
                "Deleted cancelled event with UID {uid}"
            )));
        }
        let status = match new_parent_sequence {
            Some(_) if had_occurrences => {
                format!("Updated parent event while preserving instances with UID {uid}")
            }
            Some(sequence) => format!("Updated event with UID {uid}, new sequence: {sequence}"),
            None if occurrences_changed > 0 => {
                format!("Updated recurring event instance with UID {uid}")
            }
            None if occurrences_removed > 0 => {
                format!("Removed cancelled occurrence from event with UID {uid}")
            }
            None => {
                let (new, stored) = skipped.unwrap_or_default();
                return Ok(Action::Nothing(format!(
                    "Not processing older event (sequence: {new} vs {stored}, DTSTAMP comparison) with UID {uid}"
                )));
            }
        };
        Ok(Action::Store(merged, status))
    }

    /// METHOD:REPLY: sets the replying attendee's status in the stored event.
    /// A reply only ever changes that one attendee.
    fn apply_reply(
        &self,
        uid: &str,
        reply: &Calendar,
        stored: Option<&Calendar>,
    ) -> std::result::Result<Action, Failure> {
        let Some(stored) = stored else {
            // A reply is not an event. Stored as one, it would also keep the
            // real invitation out if that is processed later.
            if !self.config.force {
                return Ok(Action::Nothing(format!(
                    "Ignoring reply for unknown event with UID {uid}"
                )));
            }
            return Ok(Action::Store(
                reply.clone(),
                format!("Stored new reply event with UID {uid}"),
            ));
        };

        let invalid = |reason: &str| {
            fail(
                format!("Invalid reply for event with UID {uid}"),
                Error::new(format!("reply for event {uid}: {reason}")),
            )
        };
        let mut merged = stored.clone();
        let mut added = None;
        let mut applied = false;

        for reply_event in reply.events() {
            let attendee = reply_event
                .prop("ATTENDEE")
                .ok_or_else(|| invalid("no ATTENDEE property in reply"))?;
            let status = attendee
                .param("PARTSTAT")
                .filter(|status| !status.is_empty())
                .ok_or_else(|| invalid("no PARTSTAT found in reply"))?;
            let comment = reply_event.value("COMMENT").and_then(response_comment);

            // The reply is for the parent event, or for the occurrence it names.
            let recurrence_id = reply_event.value("RECURRENCE-ID");
            let Some(target) = merged
                .events_mut()
                .find(|event| event.value("RECURRENCE-ID") == recurrence_id)
            else {
                continue;
            };
            applied = true;

            // Addresses are compared without regard to case: `MAILTO:` and
            // `mailto:` are the same, and Outlook sends the former.
            let stored_attendee = target
                .properties
                .iter_mut()
                .find(|p| p.name == "ATTENDEE" && p.value.eq_ignore_ascii_case(&attendee.value));
            match stored_attendee {
                Some(stored_attendee) => {
                    stored_attendee.set_param("PARTSTAT", status);
                    set_response_comment(stored_attendee, comment);
                }
                // Someone who was not invited (a delegate, an alias, a
                // forwarded invitation) is added as an attendee.
                None => {
                    let mut new_attendee = attendee.clone();
                    set_response_comment(&mut new_attendee, comment);
                    let after_attendees = target
                        .properties
                        .iter()
                        .rposition(|p| p.name == "ATTENDEE")
                        .map_or(target.properties.len(), |index| index + 1);
                    target.properties.insert(after_attendees, new_attendee);
                    added = Some(attendee.value.clone());
                }
            }
        }

        if !applied {
            return Ok(Action::Nothing(format!(
                "Ignoring reply for an occurrence that is not stored, event with UID {uid}"
            )));
        }
        let status = match added {
            Some(address) => format!("Added attendee {address} to event with UID {uid}"),
            None => format!("Updated attendee status for event with UID {uid}"),
        };
        Ok(Action::Store(merged, status))
    }
}

fn fail(status: impl Into<String>, error: Error) -> Failure {
    Failure {
        status: status.into(),
        error,
    }
}

fn is_occurrence(component: &Component) -> bool {
    component.name == "VEVENT" && component.has_prop("RECURRENCE-ID")
}

fn mark_cancelled(event: &mut Component) {
    event.set_prop("STATUS", "CANCELLED");
}

/// Splits a calendar into one calendar per UID, in the order the UIDs appear.
/// Each keeps the calendar properties and all components that are not events
/// (time zones).
fn split_by_uid(calendar: &Calendar) -> Result<Vec<(String, Calendar)>> {
    let mut uids: Vec<&str> = Vec::new();
    for event in calendar.events() {
        let uid = event
            .value("UID")
            .ok_or_else(|| Error::new("parsing email: VEVENT missing required UID property"))?;
        if !uids.contains(&uid) {
            uids.push(uid);
        }
    }
    Ok(uids
        .into_iter()
        .map(|uid| {
            let mut events = calendar.clone();
            events
                .components
                .retain(|c| c.name != "VEVENT" || c.value("UID") == Some(uid));
            (uid.to_string(), events)
        })
        .collect())
}

/// Repairs and sanitizes calendar data from a mail before it is used.
fn normalize(calendar: &mut Calendar, received: DateTime<Utc>, config: &ProcessorConfig) {
    // DTSTAMP is required, but Outlook leaves it out of changed occurrences
    // (test 23). Use the one of another event in the mail, else the mail's date.
    let dtstamp = calendar
        .events()
        .find_map(|event| event.value("DTSTAMP"))
        .map(str::to_string)
        .unwrap_or_else(|| received.format("%Y%m%dT%H%M%SZ").to_string());
    for event in calendar.events_mut() {
        if !event.has_prop("DTSTAMP") {
            event.set_prop("DTSTAMP", &dtstamp);
        }
        if config.drop_alarms {
            event.components.retain(|c| c.name != "VALARM");
        }
    }
    sanitize(calendar);
}

/// Removes what a sender must not be able to put into the calendar.
fn sanitize(component: &mut Component) {
    for property in &mut component.properties {
        match property.name.as_str() {
            // A calendar client that later sends the event on would read the
            // local file and attach it. Like Evolution, keep the reference
            // but make it unusable.
            "ATTACH" if has_prefix(&property.value, "file:") => {
                property.value.insert(0, 'x');
            }
            // These ask the server to send scheduling messages, or report
            // that it did. They do not belong in a mail (RFC 6638, 7.2, 7.3).
            "ORGANIZER" | "ATTENDEE" => {
                property.remove_param("SCHEDULE-FORCE-SEND");
                property.remove_param("SCHEDULE-STATUS");
            }
            _ => {}
        }
    }
    component.components.iter_mut().for_each(sanitize);
}

fn has_prefix(text: &str, prefix: &str) -> bool {
    text.len() >= prefix.len() && text[..prefix.len()].eq_ignore_ascii_case(prefix)
}

/// Adds the VTIMEZONE components of `other` that `calendar` does not have.
fn add_missing_time_zones(calendar: &mut Calendar, other: &Calendar) {
    for zone in other.components.iter().filter(|c| c.name == "VTIMEZONE") {
        let known = calendar
            .components
            .iter()
            .any(|c| c.name == "VTIMEZONE" && c.value("TZID") == zone.value("TZID"));
        if !known {
            let before_events = calendar
                .components
                .iter()
                .position(|c| c.name == "VEVENT")
                .unwrap_or(calendar.components.len());
            calendar.components.insert(before_events, zone.clone());
        }
    }
}

/// Removes an occurrence from the series by adding an EXDATE for it to the
/// parent event.
fn exclude_occurrence(calendar: &mut Calendar, occurrence: &Component) {
    let Some(recurrence_id) = occurrence.prop("RECURRENCE-ID") else {
        return;
    };
    let Some(parent) = calendar.events_mut().find(|event| !is_occurrence(event)) else {
        return;
    };
    let already_excluded = parent.props("EXDATE").any(|exdate| {
        exdate
            .value
            .split(',')
            .any(|date| date == recurrence_id.value)
    });
    if !already_excluded {
        let mut exdate = Property::new("EXDATE", &recurrence_id.value);
        exdate.params = recurrence_id
            .params
            .iter()
            .filter(|(key, _)| !key.eq_ignore_ascii_case("RANGE"))
            .cloned()
            .collect();
        parent.properties.push(exdate);
    }
}

/// A reply's comment as a parameter value: one line, without quotes.
fn response_comment(comment: &str) -> Option<String> {
    let comment: String = comment
        .chars()
        .map(|c| match c {
            '"' => '\'',
            c if c.is_control() => ' ',
            c => c,
        })
        .collect();
    let comment = comment.split_whitespace().collect::<Vec<_>>().join(" ");
    (!comment.is_empty()).then_some(comment)
}

/// Stores the comment of a reply with the attendee, where Evolution and
/// Google Calendar look for it. A reply without comment clears an old one.
fn set_response_comment(attendee: &mut Property, comment: Option<String>) {
    match comment {
        Some(comment) => attendee.set_param("X-RESPONSE-COMMENT", &comment),
        None => attendee.remove_param("X-RESPONSE-COMMENT"),
    }
}

/// Prepares an event for the CalDAV server. Every stored event passes through
/// here. `stored` is the event it replaces, if there is one.
///
/// - Removes the METHOD property, which belongs to the mail transport.
/// - Sets the SCHEDULE-AGENT parameter of every ORGANIZER and ATTENDEE; see
///   [`set_schedule_agents`].
fn prepare_event_for_storage(
    uid: &str,
    mut calendar: Calendar,
    stored: Option<&Calendar>,
) -> Result<Event> {
    calendar.remove_props("METHOD");
    set_schedule_agents(&mut calendar, stored);
    let raw_data = ical::encode(&calendar).context("encoding calendar")?;
    Ok(Event {
        uid: uid.to_string(),
        ..ical::parse_ical_data(&raw_data)?
    })
}

/// Tells a CalDAV server with scheduling support (RFC 6638) not to send its
/// own invitations for an event that was already exchanged by mail.
///
/// - A participant that is new on the server gets `SCHEDULE-AGENT=CLIENT`:
///   every participant of a new event, and participants added to a stored one.
/// - A participant that is already in the stored event keeps the stored
///   value, including "no parameter". Changing it is not neutral: if the
///   calendar owner is the organizer, a server must send the attendee a
///   CANCEL when an attendee goes from the default (`SERVER`) to `CLIENT`
///   (RFC 6638, section 3.2.1.2). Evolution has the same rule.
fn set_schedule_agents(calendar: &mut Component, stored: Option<&Component>) {
    const PARAM: &str = "SCHEDULE-AGENT";
    let is_participant = |p: &Property| p.name == "ORGANIZER" || p.name == "ATTENDEE";

    let stored_participants: Vec<&Property> = stored
        .iter()
        .flat_map(|stored| stored.events())
        .flat_map(|vevent| vevent.properties.iter())
        .filter(|p| is_participant(p))
        .collect();

    for vevent in calendar.events_mut() {
        for participant in vevent.properties.iter_mut().filter(|p| is_participant(p)) {
            let stored_participant = stored_participants.iter().find(|stored| {
                stored.name == participant.name
                    && stored.value.eq_ignore_ascii_case(&participant.value)
            });
            match stored_participant.map(|stored| stored.param(PARAM)) {
                Some(Some(agent)) => participant.set_param(PARAM, agent),
                Some(None) => participant.remove_param(PARAM),
                None => participant.set_param(PARAM, "CLIENT"),
            }
        }
    }
}
