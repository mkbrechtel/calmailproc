//! Storage for calendar events, keyed by UID.
//!
//! Storage holds no business logic: deciding whether an event may replace a
//! stored one is the processor's job.

mod caldav;
mod memory;

pub use caldav::{CalDavStorage, WebdavConfig};
pub use memory::MemoryStorage;

use crate::error::Result;
use crate::ical::Event;

pub trait Storage {
    /// Stores an event, replacing any event with the same UID.
    fn store_event(&mut self, event: &Event) -> Result<()>;

    /// Returns the event with this UID, or `None` if there is none.
    fn get_event(&mut self, uid: &str) -> Result<Option<Event>>;
}
