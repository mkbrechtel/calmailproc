package memory

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/emersion/go-ical"
	"github.com/mkbrechtel/calmailproc/parser"
)

// MemoryStorage implements the storage.Storage interface using an in-memory map
// Primarily intended for testing
type MemoryStorage struct {
	events map[string]*parser.CalendarEvent
	mu     sync.RWMutex
}

// NewMemoryStorage creates a new in-memory storage
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		events: make(map[string]*parser.CalendarEvent),
	}
}

// StoreEvent stores a calendar event in memory
func (s *MemoryStorage) StoreEvent(event *parser.CalendarEvent) error {
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
		eventCopy := &parser.CalendarEvent{
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
	existingCal, err := ical.NewDecoder(bytes.NewReader(existingEvent.RawData)).Decode()
	if err != nil {
		// Can't parse existing, overwrite with new data
		s.events[event.UID] = event
		return nil
	}

	// Parse the new event data
	newCal, err := ical.NewDecoder(bytes.NewReader(event.RawData)).Decode()
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

	// Check if this is a modification to a specific occurrence (has RECURRENCE-ID)
	recurrenceID := newEvent.Props.Get("RECURRENCE-ID")
	if recurrenceID != nil {
		// This is an update to a specific occurrence
		return s.handleRecurringEventUpdate(existingCal, newEvent, methodValue, event.UID)
	}

	// This is a new master event or a complete update
	// Just replace the existing event
	s.events[event.UID] = event
	return nil
}

// handleRecurringEventUpdate merges a recurring event update into the existing calendar
func (s *MemoryStorage) handleRecurringEventUpdate(existingCal *ical.Calendar, newEvent *ical.Component, methodValue string, uid string) error {
	recurrenceID := newEvent.Props.Get("RECURRENCE-ID")
	if recurrenceID == nil {
		return fmt.Errorf("missing RECURRENCE-ID in event update")
	}

	// Find if this specific occurrence already exists in the calendar
	foundExisting := false
	for i, component := range existingCal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		// Check if this is the same occurrence by matching RECURRENCE-ID
		existingRecurrenceID := component.Props.Get("RECURRENCE-ID")
		if existingRecurrenceID != nil && existingRecurrenceID.Value == recurrenceID.Value {
			// Found the existing occurrence to update
			foundExisting = true

			// Handle cancellations (METHOD:CANCEL)
			if methodValue == "CANCEL" {
				// For cancellations, we update the status to CANCELLED
				component.Props.Set(&ical.Prop{Name: "STATUS", Value: "CANCELLED"})
			} else {
				// Replace the existing occurrence with the new one
				existingCal.Children[i] = newEvent
			}
			break
		}
	}

	// If we didn't find an existing occurrence with this RECURRENCE-ID, add it
	if !foundExisting {
		if methodValue == "CANCEL" {
			// For cancellations of instances we haven't seen before, create a new component with STATUS:CANCELLED
			newEvent.Props.Set(&ical.Prop{Name: "STATUS", Value: "CANCELLED"})
			existingCal.Children = append(existingCal.Children, newEvent)
		} else {
			// Add the new occurrence
			existingCal.Children = append(existingCal.Children, newEvent)
		}
	}

	// Encode the updated calendar back to bytes
	var buf bytes.Buffer
	encoder := ical.NewEncoder(&buf)
	if err := encoder.Encode(existingCal); err != nil {
		return fmt.Errorf("encoding updated calendar: %w", err)
	}

	// Update the event in memory
	s.events[uid].RawData = buf.Bytes()
	return nil
}

// GetEvent retrieves a calendar event from memory
func (s *MemoryStorage) GetEvent(id string) (*parser.CalendarEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	event, ok := s.events[id]
	if !ok {
		return nil, fmt.Errorf("event not found: %s", id)
	}

	// Return a copy to prevent modification of internal state
	eventCopy := &parser.CalendarEvent{
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
func (s *MemoryStorage) ListEvents() ([]*parser.CalendarEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := make([]*parser.CalendarEvent, 0, len(s.events))
	for _, event := range s.events {
		// Create a copy of each event
		eventCopy := &parser.CalendarEvent{
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
	s.events = make(map[string]*parser.CalendarEvent)
}
