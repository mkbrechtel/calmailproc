package storage

import (
	"fmt"
	"sync"

	"github.com/mkbrechtel/calmailproc/parser/ical"
)

type MemoryStorage struct {
	events map[string]*ical.Event
	mu     sync.RWMutex
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		events: make(map[string]*ical.Event),
	}
}

func (m *MemoryStorage) StoreEvent(event *ical.Event) error {
	if event.UID == "" {
		return fmt.Errorf("event has no UID")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.events[event.UID] = event
	return nil
}

func (m *MemoryStorage) GetEvent(uid string) (*ical.Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	event, ok := m.events[uid]
	if !ok {
		return nil, fmt.Errorf("event not found")
	}
	return event, nil
}

func (m *MemoryStorage) ListEvents() ([]*ical.Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]*ical.Event, 0, len(m.events))
	for _, event := range m.events {
		events = append(events, event)
	}
	return events, nil
}

func (m *MemoryStorage) DeleteEvent(uid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.events, uid)
	return nil
}

func (m *MemoryStorage) GetEventCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.events)
}