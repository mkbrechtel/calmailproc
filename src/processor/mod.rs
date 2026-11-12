//! The processor: decides what happens to the calendar event in an email.
//!
//! It orchestrates the email parser, the iCalendar module and storage, and
//! holds all business logic: which version of an event wins, how updates to
//! single occurrences of a recurring event are merged, and how attendance
//! replies are applied.

pub mod maildir;
pub mod stdin;

use chrono::Utc;
use serde::Deserialize;

use crate::email::{self, Email};
use crate::error::{Context, Error, Result};
use crate::ical::{self, Comparison, Component, Event};
use crate::storage::Storage;

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct ProcessorConfig {
    pub process_replies: bool,
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

pub struct Processor<S: Storage> {
    pub storage: S,
    pub process_replies: bool,
}

impl<S: Storage> Processor<S> {
    pub fn new(storage: S, process_replies: bool) -> Self {
        Processor {
            storage,
            process_replies,
        }
    }

    pub fn from_config(storage: S, config: &ProcessorConfig) -> Self {
        Self::new(storage, config.process_replies)
    }

    /// Processes one raw email and returns a status message.
    pub fn process_email(&mut self, raw: &[u8]) -> Outcome {
        let parsed: Email = email::parse(raw)
            .context("parsing email")
            .map_err(|e| fail("E-Mail parsing error", e))?;

        let Some(event) = parsed.event.filter(|event| !event.uid.is_empty()) else {
            return Ok(NO_CALENDAR_EVENT.to_string());
        };
        ical::validate_uid(&event.uid)
            .map_err(|e| fail(format!("Invalid UID for calendar event: {e}"), e))?;

        match event.method.as_str() {
            "REPLY" => self.process_event_reply(&event),
            // REQUEST, CANCEL and everything else (e.g. PUBLISH) are handled alike.
            _ => self.process_event(&event),
        }
    }

    /// Stores a new event or merges it with the stored one.
    fn process_event(&mut self, event: &Event) -> Outcome {
        let uid = &event.uid;
        ical::validate_event(&event.raw_data)
            .context(&format!("validation error for event {uid}"))
            .map_err(|e| fail(format!("Invalid calendar data for event with UID {uid}"), e))?;

        let Some(existing) = self.get_existing(uid)? else {
            self.store(event, "Error storing new event", "storing event")?;
            return Ok(format!("Stored new event with UID {uid}"));
        };

        // An update to one occurrence is always merged, whatever the sequence
        // number of the stored parent event.
        if event.is_recurring_update() {
            let updated = handle_recurring_event(&existing, event)
                .context("handling recurring instance")
                .map_err(|e| fail("Error handling recurring instance", e))?;
            self.validate_and_store(&updated, "instance update")?;
            return Ok(format!("Updated recurring event instance with UID {uid}"));
        }

        // The stored data already has occurrence updates: replace the parent
        // event and keep them.
        if existing.is_recurring_update() {
            let updated = handle_parent_event_update(&existing, event)
                .context("handling parent event update")
                .map_err(|e| fail("Error handling parent event update", e))?;
            self.validate_and_store(&updated, "parent update")?;
            return Ok(format!(
                "Updated parent event while preserving instances with UID {uid}"
            ));
        }

        let comparison = ical::compare_events(event, &existing)
            .context("comparing events")
            .map_err(|e| fail("Error comparing events", e))?;
        if comparison == Comparison::SecondNewer {
            return Ok(format!(
                "Not processing older event (sequence: {} vs {}, DTSTAMP comparison) with UID {uid}",
                event.sequence, existing.sequence
            ));
        }

        let updated = handle_parent_event_update(&existing, event)
            .context("handling parent event update")
            .map_err(|e| fail("Error handling parent event update", e))?;
        self.validate_and_store(&updated, "parent update")?;
        Ok(format!(
            "Updated event with UID {uid}, new sequence: {}",
            event.sequence
        ))
    }

    /// Applies an attendance reply (METHOD:REPLY) to the stored event.
    fn process_event_reply(&mut self, reply: &Event) -> Outcome {
        if !self.process_replies {
            return Ok("Ignoring calendar REPLY method as configured".to_string());
        }
        let uid = &reply.uid;
        ical::validate_event(&reply.raw_data)
            .context(&format!("validation error for event reply {uid}"))
            .map_err(|e| {
                fail(
                    format!("Invalid calendar data for event reply with UID {uid}"),
                    e,
                )
            })?;

        let Some(mut existing) = self.get_existing(uid)? else {
            self.store(reply, "Error storing new reply event", "storing event")?;
            return Ok(format!("Stored new reply event with UID {uid}"));
        };

        if let Err(error) = update_attendee_status(reply, &mut existing) {
            eprintln!("Warning: Failed to update attendee status: {error}");
            self.store(reply, "Error storing reply event", "storing event")?;
            return Ok(format!(
                "Stored reply event with UID {uid} (attendee update failed)"
            ));
        }

        ical::validate_event(&existing.raw_data)
            .context(&format!(
                "validation error after attendee update for event {uid}"
            ))
            .map_err(|e| {
                fail(
                    format!("Invalid calendar data after attendee update for event with UID {uid}"),
                    e,
                )
            })?;
        self.store(
            &existing,
            "Error storing updated event with attendee status",
            "storing updated event",
        )?;
        Ok(format!("Updated attendee status for event with UID {uid}"))
    }

    fn get_existing(&mut self, uid: &str) -> std::result::Result<Option<Event>, Failure> {
        self.storage
            .get_event(uid)
            .context("looking up existing event")
            .map_err(|e| fail("Error looking up existing event", e))
    }

    fn validate_and_store(
        &mut self,
        event: &Event,
        step: &str,
    ) -> std::result::Result<(), Failure> {
        let uid = &event.uid;
        ical::validate_event(&event.raw_data)
            .context(&format!("validation error after {step} for event {uid}"))
            .map_err(|e| {
                fail(
                    format!("Invalid calendar data after {step} for event with UID {uid}"),
                    e,
                )
            })?;
        self.store(
            event,
            "Error storing updated event",
            "storing updated event",
        )
    }

    fn store(
        &mut self,
        event: &Event,
        status: &str,
        context: &str,
    ) -> std::result::Result<(), Failure> {
        let prepared = prepare_event_for_storage(event)
            .context("preparing event")
            .map_err(|e| fail("Error preparing event for storage", e))?;
        self.storage
            .store_event(&prepared)
            .context(context)
            .map_err(|e| fail(status, e))
    }
}

fn fail(status: impl Into<String>, error: Error) -> Failure {
    Failure {
        status: status.into(),
        error,
    }
}

/// Merges an update to one occurrence (a VEVENT with RECURRENCE-ID) into the
/// stored calendar: replaces the occurrence if it is already there, adds it
/// otherwise. A cancellation marks the occurrence as cancelled.
fn handle_recurring_event(existing: &Event, new: &Event) -> Result<Event> {
    let new_cal = ical::decode(&new.raw_data).context("parsing new event data")?;
    let mut existing_cal =
        ical::decode(&existing.raw_data).context("parsing existing event data")?;

    let mut occurrence = new_cal
        .events()
        .find(|e| e.has_prop("RECURRENCE-ID"))
        .cloned()
        .ok_or_else(|| Error::new("no recurring event found in update"))?;
    let recurrence_id = occurrence
        .value("RECURRENCE-ID")
        .unwrap_or_default()
        .to_string();
    let is_cancel = new.method == "CANCEL";

    let stored = existing_cal
        .events_mut()
        .find(|e| e.value("RECURRENCE-ID") == Some(recurrence_id.as_str()));
    match stored {
        Some(stored) if is_cancel => stored.set_prop("STATUS", "CANCELLED"),
        Some(stored) => *stored = occurrence,
        None => {
            if is_cancel {
                occurrence.set_prop("STATUS", "CANCELLED");
            }
            existing_cal.components.push(occurrence);
        }
    }

    // DTSTAMP is required; add it where a sender left it out.
    let now = Utc::now().format("%Y%m%dT%H%M%SZ").to_string();
    for event in existing_cal.events_mut() {
        if !event.has_prop("DTSTAMP") {
            event.set_prop("DTSTAMP", &now);
        }
    }

    Ok(Event {
        uid: existing.uid.clone(),
        raw_data: ical::encode(&existing_cal).context("encoding updated calendar")?,
        summary: existing.summary.clone(),
        method: existing.method.clone(),
        sequence: existing.sequence,
    })
}

/// Replaces the parent event with the new one and keeps the stored occurrence
/// updates (VEVENTs with RECURRENCE-ID).
fn handle_parent_event_update(existing: &Event, new: &Event) -> Result<Event> {
    let new_cal = ical::decode(&new.raw_data).context("parsing new parent event data")?;
    let existing_cal = ical::decode(&existing.raw_data).context("parsing existing event data")?;

    let is_occurrence = |c: &Component| c.name == "VEVENT" && c.has_prop("RECURRENCE-ID");
    if !new_cal.events().any(|e| !is_occurrence(e)) {
        return Err(Error::new("no parent event component found in update"));
    }

    // Take the new calendar (properties, time zones, first parent event) and
    // append the stored occurrence updates.
    let mut updated = new_cal;
    let mut parent_seen = false;
    updated.components.retain(|c| {
        if c.name != "VEVENT" {
            return true;
        }
        let keep = !is_occurrence(c) && !parent_seen;
        parent_seen |= keep;
        keep
    });
    updated.components.extend(
        existing_cal
            .components
            .into_iter()
            .filter(|c| is_occurrence(c)),
    );

    Ok(Event {
        uid: existing.uid.clone(),
        raw_data: ical::encode(&updated).context("encoding updated calendar")?,
        summary: new.summary.clone(),
        method: new.method.clone(),
        sequence: new.sequence,
    })
}

/// Sets the replying attendee's PARTSTAT in the stored event.
fn update_attendee_status(reply: &Event, stored: &mut Event) -> Result<()> {
    let reply_cal = ical::decode(&reply.raw_data).context("parsing event data")?;
    let mut stored_cal = ical::decode(&stored.raw_data).context("parsing existing event data")?;

    let reply_event = reply_cal
        .events()
        .next()
        .ok_or_else(|| Error::new("no VEVENT component found in reply"))?;
    let attendee = reply_event
        .prop("ATTENDEE")
        .ok_or_else(|| Error::new("no ATTENDEE property in reply"))?;
    let status = attendee
        .param("PARTSTAT")
        .filter(|status| !status.is_empty())
        .ok_or_else(|| Error::new("no PARTSTAT found in reply"))?;

    // The reply applies to the parent event, or to the occurrence it names.
    // Addresses are compared without regard to case: `MAILTO:` and `mailto:`
    // are the same, and Outlook sends the former.
    let stored_attendee = stored_cal
        .events_mut()
        .filter(|event| matches_recurrence_id(reply_event, event))
        .flat_map(|event| event.properties.iter_mut())
        .find(|p| p.name == "ATTENDEE" && p.value.eq_ignore_ascii_case(&attendee.value))
        .ok_or_else(|| Error::new(format!("attendee {} not found in event", attendee.value)))?;
    stored_attendee.set_param("PARTSTAT", status);

    stored.raw_data = ical::encode(&stored_cal).context("encoding updated calendar")?;
    Ok(())
}

/// True if both components are the parent event, or both are the same
/// occurrence.
fn matches_recurrence_id(a: &Component, b: &Component) -> bool {
    a.value("RECURRENCE-ID") == b.value("RECURRENCE-ID")
}

/// Removes the METHOD property, which belongs to the mail transport and must
/// not be stored.
fn prepare_event_for_storage(event: &Event) -> Result<Event> {
    let mut calendar = ical::decode(&event.raw_data).context("decoding calendar")?;
    calendar.remove_props("METHOD");
    Ok(Event {
        raw_data: ical::encode(&calendar).context("encoding calendar")?,
        ..event.clone()
    })
}
