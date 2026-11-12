//! In-memory storage, used by the tests. It checks versions like a CalDAV
//! server checks ETags.

use std::collections::BTreeMap;

use super::{Storage, StoreError, Stored, Version};
use crate::error::{Error, Result};
use crate::ical::Event;

#[derive(Debug, Default)]
pub struct MemoryStorage {
    events: BTreeMap<String, (Event, u64)>,
    writes: u64,
    /// The next this many writes fail with a conflict, as if another writer
    /// had changed the event in between. For tests.
    pub conflicts_to_inject: u32,
}

impl MemoryStorage {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn list_events(&self) -> Vec<&Event> {
        self.events.values().map(|(event, _)| event).collect()
    }

    pub fn event_count(&self) -> usize {
        self.events.len()
    }

    /// The event with this UID, without version information.
    pub fn event(&self, uid: &str) -> Option<&Event> {
        self.events.get(uid).map(|(event, _)| event)
    }

    /// Puts an event into storage unconditionally, as another writer would.
    pub fn put(&mut self, event: &Event) {
        self.writes += 1;
        self.events
            .insert(event.uid.clone(), (event.clone(), self.writes));
    }

    fn check(
        &mut self,
        uid: &str,
        expected: Option<&Version>,
    ) -> std::result::Result<(), StoreError> {
        if self.conflicts_to_inject > 0 {
            self.conflicts_to_inject -= 1;
            return Err(StoreError::Conflict);
        }
        let current = self.events.get(uid).map(|(_, version)| version.to_string());
        match (current, expected.and_then(|version| version.etag.clone())) {
            (None, None) => Ok(()),
            (Some(current), Some(expected)) if current == expected => Ok(()),
            _ => Err(StoreError::Conflict),
        }
    }
}

impl Storage for MemoryStorage {
    fn get_event(&mut self, uid: &str) -> Result<Option<Stored>> {
        Ok(self.events.get(uid).map(|(event, version)| Stored {
            event: event.clone(),
            version: Version {
                location: uid.to_string(),
                etag: Some(version.to_string()),
            },
        }))
    }

    fn store_event(
        &mut self,
        event: &Event,
        replaces: Option<&Version>,
    ) -> std::result::Result<(), StoreError> {
        if event.uid.is_empty() {
            return Err(Error::new("event has no UID").into());
        }
        self.check(&event.uid, replaces)?;
        self.put(event);
        Ok(())
    }

    fn delete_event(&mut self, version: &Version) -> std::result::Result<(), StoreError> {
        self.check(&version.location, Some(version))?;
        self.events.remove(&version.location);
        Ok(())
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
        assert!(store.get_event("a").unwrap().is_none());

        store.store_event(&event("a", "first"), None).unwrap();
        store.store_event(&event("b", "other"), None).unwrap();
        let stored = store.get_event("a").unwrap().unwrap();
        store
            .store_event(&event("a", "second"), Some(&stored.version))
            .unwrap();
        assert_eq!(store.event_count(), 2);
        assert_eq!(store.event("a").unwrap().summary, "second");
        assert_eq!(store.list_events().len(), 2);

        let stored = store.get_event("a").unwrap().unwrap();
        store.delete_event(&stored.version).unwrap();
        assert_eq!(store.event_count(), 1);
        assert!(store.store_event(&event("", "no uid"), None).is_err());
    }

    #[test]
    fn writes_against_an_outdated_version_conflict() {
        let mut store = MemoryStorage::new();
        store.store_event(&event("a", "first"), None).unwrap();

        // Creating an event that already exists.
        assert_eq!(
            store.store_event(&event("a", "again"), None),
            Err(StoreError::Conflict)
        );

        // Someone else writes between our read and our write.
        let read = store.get_event("a").unwrap().unwrap();
        store.put(&event("a", "theirs"));
        assert_eq!(
            store.store_event(&event("a", "ours"), Some(&read.version)),
            Err(StoreError::Conflict)
        );
        assert_eq!(store.delete_event(&read.version), Err(StoreError::Conflict));
        assert_eq!(store.event("a").unwrap().summary, "theirs");
    }
}
