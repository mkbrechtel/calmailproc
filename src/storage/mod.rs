//! Storage for calendar events, keyed by UID.
//!
//! Storage holds no business logic: deciding whether an event may replace a
//! stored one is the processor's job. Storage does guard against lost
//! updates: an event is written back only if it is still the version that
//! was read.

mod caldav;
mod memory;

pub use caldav::{CalDavStorage, WebdavConfig};
pub use memory::MemoryStorage;

use std::fmt;

use crate::error::{Error, Result};
use crate::ical::Event;

/// Where a stored event lives and which version of it was read. Opaque to
/// the processor, which only hands it back when writing.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Version {
    pub location: String,
    pub etag: Option<String>,
}

/// An event as read from storage.
#[derive(Debug, Clone)]
pub struct Stored {
    pub event: Event,
    pub version: Version,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum StoreError {
    /// The stored event changed since it was read, or an event that was
    /// expected to be new already exists. Read again and retry.
    Conflict,
    Other(Error),
}

impl fmt::Display for StoreError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            StoreError::Conflict => f.write_str("the stored event was changed by someone else"),
            StoreError::Other(error) => error.fmt(f),
        }
    }
}

impl From<Error> for StoreError {
    fn from(error: Error) -> Self {
        StoreError::Other(error)
    }
}

pub trait Storage {
    /// Returns the event with this UID, wherever it is stored, or `None`.
    fn get_event(&mut self, uid: &str) -> Result<Option<Stored>>;

    /// Stores an event. `replaces` is the version read by [`get_event`] that
    /// this event replaces, or `None` if the event must be new.
    ///
    /// [`get_event`]: Storage::get_event
    fn store_event(
        &mut self,
        event: &Event,
        replaces: Option<&Version>,
    ) -> std::result::Result<(), StoreError>;

    /// Deletes the stored event that was read as `version`.
    fn delete_event(&mut self, version: &Version) -> std::result::Result<(), StoreError>;
}
