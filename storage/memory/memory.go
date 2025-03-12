package memory

import (
	"fmt"
	"sync"

	"github.com/mkbrechtel/calmailproc/manager"
	"github.com/mkbrechtel/calmailproc/parser/ical"
)

// MemoryStorage implements the storage.Storage interface using an in-memory map
// Primarily intended for testing
type MemoryStorage struct {
	events         map[string]*ical.Event
	mu             sync.RWMutex
	calendarManager manager.Calendar
}

// NewMemoryStorage creates a new in-memory storage
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		events:         make(map[string]*ical.Event),
		calendarManager: manager.NewDefaultManager(),
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

	// Check if we already have this event to handle recurring event updates
	existingEvent, exists := s.events[event.UID]
	if !exists {
		// New event, just store it
		// Make a copy to avoid potential references issues
		eventCopy := &ical.Event{
			UID:         event.UID,
			RawData:     make([]byte, len(event.RawData)),
			Summary:     event.Summary,
			Start:       event.Start,
			End:         event.End,
			Location:    event.Location,
			Organizer:   event.Organizer,
			Description: event.Description,
			Method:      event.Method,
		}
		copy(eventCopy.RawData, event.RawData)
		s.events[event.UID] = eventCopy
		return nil
	}

	// Parse the existing event data
	existingCal, err := ical.DecodeCalendar(existingEvent.RawData)
	if err != nil {
		// Can't parse existing, overwrite with new data
		s.events[event.UID] = event
		return nil
	}

	// Parse the new event data
	newCal, err := ical.DecodeCalendar(event.RawData)
	if err != nil {
		// Can't parse new data, keep existing
		return fmt.Errorf("parsing new event data: %w", err)
	}

	// Extract the METHOD if present
	methodValue := ""
	methodProp := newCal.Props.Get("METHOD")
	if methodProp != nil {
		methodValue = methodProp.Value
	}

	// Extract the current event from the new data
	var newEvent *ical.Component
	for _, component := range newCal.Children {
		if component.Name == "VEVENT" {
			newEvent = component
			break
		}
	}
	if newEvent == nil {
		return fmt.Errorf("no VEVENT component found in calendar data")
	}

	// Handle METHOD:REPLY for attendee updates
	if event.Method == "REPLY" && existingEvent != nil {
		// Try to update attendee status
		if err := s.calendarManager.UpdateAttendeeStatus(event, existingEvent); err != nil {
			// Log the error but continue with regular processing
			fmt.Printf("Warning: Failed to update attendee status: %v\n", err)
		} else {
			// Successfully updated attendee status, we're done
			return nil
		}
	}
	
	// Check if this is a modification to a specific occurrence (has RECURRENCE-ID)
	recurrenceID := newEvent.Props.Get("RECURRENCE-ID")
	if recurrenceID != nil {
		// This is an update to a specific occurrence
		updatedCal, err := s.calendarManager.HandleRecurringEventUpdate(existingCal, newEvent, methodValue)
		if err != nil {
			return fmt.Errorf("handling recurring event update: %w", err)
		}
		
		// Encode the updated calendar back to bytes
		calBytes, err := ical.EncodeCalendar(updatedCal)
		if err != nil {
			return fmt.Errorf("encoding updated calendar: %w", err)
		}
		
		// Update the event in memory
		s.events[event.UID].RawData = calBytes
		return nil
	}

	// This is a new master event or a complete update
	// Just replace the existing event
	s.events[event.UID] = event
	return nil
}

// GetEvent retrieves a calendar event from memory
func (s *MemoryStorage) GetEvent(id string) (*ical.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	event, ok := s.events[id]
	if !ok {
		return nil, fmt.Errorf("event not found: %s", id)
	}

	// Return a copy to prevent modification of internal state
	eventCopy := &ical.Event{
		UID:         event.UID,
		RawData:     make([]byte, len(event.RawData)),
		Summary:     event.Summary,
		Start:       event.Start,
		End:         event.End,
		Location:    event.Location,
		Organizer:   event.Organizer,
		Description: event.Description,
		Method:      event.Method,
	}
	copy(eventCopy.RawData, event.RawData)

	return eventCopy, nil
}

// ListEvents lists all events in memory
func (s *MemoryStorage) ListEvents() ([]*ical.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := make([]*ical.Event, 0, len(s.events))
	for _, event := range s.events {
		// Create a copy of each event
		eventCopy := &ical.Event{
			UID:         event.UID,
			RawData:     make([]byte, len(event.RawData)),
			Summary:     event.Summary,
			Start:       event.Start,
			End:         event.End,
			Location:    event.Location,
			Organizer:   event.Organizer,
			Description: event.Description,
			Method:      event.Method,
		}
		copy(eventCopy.RawData, event.RawData)
		events = append(events, eventCopy)
	}

	return events, nil
}

// DeleteEvent deletes a calendar event from memory
func (s *MemoryStorage) DeleteEvent(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.events[id]; !ok {
		return fmt.Errorf("event not found: %s", id)
	}

	delete(s.events, id)
	return nil
}

// GetEventCount returns the number of events in memory (useful for testing)
func (s *MemoryStorage) GetEventCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

// Clear removes all events from memory (useful for testing)
func (s *MemoryStorage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = make(map[string]*ical.Event)
}