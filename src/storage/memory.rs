//! In-memory storage, used by the tests.

use std::collections::BTreeMap;

use super::Storage;
use crate::error::{Error, Result};
use crate::ical::Event;

#[derive(Debug, Default)]
pub struct MemoryStorage {
    events: BTreeMap<String, Event>,
}

impl MemoryStorage {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn list_events(&self) -> Vec<&Event> {
        self.events.values().collect()
    }

    pub fn event_count(&self) -> usize {
        self.events.len()
    }

    pub fn delete_event(&mut self, uid: &str) {
        self.events.remove(uid);
    }
}

impl Storage for MemoryStorage {
    fn store_event(&mut self, event: &Event) -> Result<()> {
        if event.uid.is_empty() {
            return Err(Error::new("event has no UID"));
        }
        self.events.insert(event.uid.clone(), event.clone());
        Ok(())
    }

    fn get_event(&mut self, uid: &str) -> Result<Option<Event>> {
        Ok(self.events.get(uid).cloned())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn event(uid: &str, summary: &str) -> Event {
        Event {
            uid: uid.to_string(),
            summary: summary.to_string(),
            ..Event::default()
        }
    }

    #[test]
    fn store_get_replace_delete() {
        let mut store = MemoryStorage::new();
        assert_eq!(store.get_event("a").unwrap(), None);

        store.store_event(&event("a", "first")).unwrap();
        store.store_event(&event("b", "other")).unwrap();
        store.store_event(&event("a", "second")).unwrap();
        assert_eq!(store.event_count(), 2);
        assert_eq!(store.get_event("a").unwrap().unwrap().summary, "second");
        assert_eq!(store.list_events().len(), 2);

        store.delete_event("a");
        assert_eq!(store.event_count(), 1);
        assert!(store.store_event(&event("", "no uid")).is_err());
    }
}
