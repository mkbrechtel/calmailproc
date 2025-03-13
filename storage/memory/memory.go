package memory

import (
	"fmt"
	"sync"

	"github.com/mkbrechtel/calmailproc/parser/ical"
)

// MemoryStorage implements the storage.Storage interface using an in-memory map
// Primarily intended for testing
type MemoryStorage struct {
	rawData map[string][]byte  // Simple key-value store with UID as key and raw data as value
	mu      sync.RWMutex
}

// NewMemoryStorage creates a new in-memory storage
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		rawData: make(map[string][]byte),
	}
}

// StoreEvent stores a calendar event in memory
func (s *MemoryStorage) StoreEvent(event *ical.Event) error {
	if event.UID == "" {
		return fmt.Errorf("event has no UID")
	}

	if len(event.RawData) == 0 {
		return fmt.Errorf("no raw calendar data to store")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Simply store the raw data by UID
	s.rawData[event.UID] = event.RawData
	return nil
}

// GetEvent retrieves a calendar event from memory
func (s *MemoryStorage) GetEvent(id string) (*ical.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rawData, ok := s.rawData[id]
	if !ok {
		return nil, fmt.Errorf("event not found: %s", id)
	}

	// Just parse the raw data to create an event object
	event, err := ical.ParseICalData(rawData)
	if err != nil {
		return nil, fmt.Errorf("parsing stored event: %w", err)
	}

	return event, nil
}

// ListEvents lists all events in memory
func (s *MemoryStorage) ListEvents() ([]*ical.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := make([]*ical.Event, 0, len(s.rawData))
	for _, data := range s.rawData {
		// Parse each raw data to create event objects
		event, err := ical.ParseICalData(data)
		if err != nil {
			// Skip events that can't be parsed
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

// DeleteEvent deletes a calendar event from memory
func (s *MemoryStorage) DeleteEvent(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rawData[id]; !ok {
		return fmt.Errorf("event not found: %s", id)
	}

	delete(s.rawData, id)
	return nil
}

// GetEventCount returns the number of events in memory (useful for testing)
func (s *MemoryStorage) GetEventCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.rawData)
}

// Clear removes all events from memory (useful for testing)
func (s *MemoryStorage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rawData = make(map[string][]byte)
}