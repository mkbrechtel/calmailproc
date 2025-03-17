package icalfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mkbrechtel/calmailproc/parser/ical"
)

// ICalFileStorage implements the storage.Storage interface using a single iCalendar file
// Note: In a dumbified implementation, this isn't suitable for production use as it will
// simply store the latest version of each event, without proper handling of recurring events.
type ICalFileStorage struct {
	FilePath string
	mu       sync.Mutex // Mutex to protect concurrent file access
}

// NewICalFileStorage creates a new ICalFileStorage with the given file path
func NewICalFileStorage(filePath string) (*ICalFileStorage, error) {
	storage := &ICalFileStorage{
		FilePath: filePath,
	}

	// Create the directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating directory for calendar file: %w", err)
	}

	return storage, nil
}

// StoreEvent stores a calendar event in the icalfile storage
// This simplified implementation just adds or replaces events in the file
// without any complex merging or validation logic
func (s *ICalFileStorage) StoreEvent(event *ical.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if event.UID == "" {
		return fmt.Errorf("event has no UID")
	}

	if len(event.RawData) == 0 {
		return fmt.Errorf("no raw calendar data to store")
	}

	// Check if file exists and read it
	var existingCal *ical.Calendar
	existingData, err := os.ReadFile(s.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create a new calendar if the file doesn't exist
			existingCal = ical.NewCalendar()
		} else {
			return fmt.Errorf("reading calendar file: %w", err)
		}
	} else {
		// Parse the existing calendar
		existingCal, err = ical.DecodeCalendar(existingData)
		if err != nil {
			// If we can't parse it, create a new calendar
			existingCal = ical.NewCalendar()
		}
	}

	// Parse the new event data to extract VEVENT components
	newCal, err := ical.DecodeCalendar(event.RawData)
	if err != nil {
		return fmt.Errorf("parsing new event data: %w", err)
	}

	// Extract the VEVENT components from the new calendar
	var newEvents []*ical.Component
	for _, component := range newCal.Children {
		if component.Name == "VEVENT" {
			newEvents = append(newEvents, component)
		}
	}

	if len(newEvents) == 0 {
		return fmt.Errorf("no VEVENT components found in the new calendar data")
	}

	// For each new event, add it or replace existing one with same UID
	for _, newEvent := range newEvents {
		uidProp := newEvent.Props.Get("UID")
		if uidProp == nil {
			continue // Skip events without UID
		}

		// Remove any existing events with the same UID
		var updatedChildren []*ical.Component
		for _, component := range existingCal.Children {
			if component.Name != "VEVENT" {
				updatedChildren = append(updatedChildren, component)
				continue
			}

			existingUID := component.Props.Get("UID")
			if existingUID == nil || existingUID.Value != uidProp.Value {
				updatedChildren = append(updatedChildren, component)
			}
		}

		// Add the new event
		updatedChildren = append(updatedChildren, newEvent)
		existingCal.Children = updatedChildren
	}

	// Write the updated calendar back to the file
	updatedData, err := ical.EncodeCalendar(existingCal)
	if err != nil {
		return fmt.Errorf("encoding updated calendar: %w", err)
	}

	if err := os.WriteFile(s.FilePath, updatedData, 0644); err != nil {
		return fmt.Errorf("writing updated calendar file: %w", err)
	}

	return nil
}

// GetEvent retrieves a calendar event from the icalfile storage
func (s *ICalFileStorage) GetEvent(id string) (*ical.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Read the calendar file
	data, err := os.ReadFile(s.FilePath)
	if err != nil {
		return nil, fmt.Errorf("reading calendar file: %w", err)
	}

	// Parse the calendar
	cal, err := ical.DecodeCalendar(data)
	if err != nil {
		return nil, fmt.Errorf("parsing calendar file: %w", err)
	}

	// Look for the specified event
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		uidProp := component.Props.Get("UID")
		if uidProp != nil && uidProp.Value == id {
			// Create a minimal calendar containing just this event
			eventCal := ical.NewCalendar()
			eventCal.Children = append(eventCal.Children, component)

			// Encode to raw bytes
			rawData, err := ical.EncodeCalendar(eventCal)
			if err != nil {
				return nil, fmt.Errorf("encoding event: %w", err)
			}

			// Create and return the event
			event, err := ical.ParseICalData(rawData)
			if err != nil {
				return nil, fmt.Errorf("parsing event data: %w", err)
			}

			return event, nil
		}
	}

	return nil, fmt.Errorf("event not found: %s", id)
}

// ListEvents lists all events in the icalfile storage
func (s *ICalFileStorage) ListEvents() ([]*ical.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Read the calendar file
	data, err := os.ReadFile(s.FilePath)
	if err != nil {
		return nil, fmt.Errorf("reading calendar file: %w", err)
	}

	// Parse the calendar
	cal, err := ical.DecodeCalendar(data)
	if err != nil {
		return nil, fmt.Errorf("parsing calendar file: %w", err)
	}

	// Extract all events
	var events []*ical.Event
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		uidProp := component.Props.Get("UID")
		if uidProp == nil {
			continue // Skip events without UID
		}

		// Create a minimal calendar containing just this event
		eventCal := ical.NewCalendar()
		eventCal.Children = append(eventCal.Children, component)

		// Encode to raw bytes
		rawData, err := ical.EncodeCalendar(eventCal)
		if err != nil {
			continue // Skip events that can't be encoded
		}

		// Create the event
		event, err := ical.ParseICalData(rawData)
		if err != nil {
			continue // Skip events that can't be parsed
		}

		events = append(events, event)
	}

	return events, nil
}

// DeleteEvent deletes a calendar event from the icalfile storage
func (s *ICalFileStorage) DeleteEvent(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Read the calendar file
	data, err := os.ReadFile(s.FilePath)
	if err != nil {
		return fmt.Errorf("reading calendar file: %w", err)
	}

	// Parse the calendar
	cal, err := ical.DecodeCalendar(data)
	if err != nil {
		return fmt.Errorf("parsing calendar file: %w", err)
	}

	// Filter out the event to delete
	found := false
	var updatedChildren []*ical.Component
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			updatedChildren = append(updatedChildren, component)
			continue
		}

		uidProp := component.Props.Get("UID")
		if uidProp == nil || uidProp.Value != id {
			updatedChildren = append(updatedChildren, component)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("event not found: %s", id)
	}

	// Update the calendar
	cal.Children = updatedChildren

	// Write the updated calendar back to the file
	updatedData, err := ical.EncodeCalendar(cal)
	if err != nil {
		return fmt.Errorf("encoding updated calendar: %w", err)
	}

	if err := os.WriteFile(s.FilePath, updatedData, 0644); err != nil {
		return fmt.Errorf("writing updated calendar file: %w", err)
	}

	return nil
}