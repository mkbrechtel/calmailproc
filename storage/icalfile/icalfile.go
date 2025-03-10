package icalfile

import (
	"bytes"
	"fmt"
	"os"
	"sync"

	"github.com/emersion/go-ical"
	"github.com/mkbrechtel/calmailproc/parser"
)

// ICalFileStorage implements the storage.Storage interface using a single iCalendar file
type ICalFileStorage struct {
	FilePath string
	mu       sync.Mutex // Mutex to protect concurrent file access
}

// NewICalFileStorage creates a new ICalFileStorage with the given file path
func NewICalFileStorage(filePath string) (*ICalFileStorage, error) {
	storage := &ICalFileStorage{
		FilePath: filePath,
	}

	// Create an empty calendar file if it doesn't exist
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Create a minimal valid iCalendar file
		initialContent := []byte("BEGIN:VCALENDAR\r\nPRODID:-//calmailproc//ICalFileStorage//EN\r\nVERSION:2.0\r\nEND:VCALENDAR\r\n")
		if err := os.WriteFile(filePath, initialContent, 0644); err != nil {
			return nil, fmt.Errorf("creating initial calendar file: %w", err)
		}
	}

	return storage, nil
}

// StoreEvent stores a calendar event in the single iCalendar file
func (s *ICalFileStorage) StoreEvent(event *parser.CalendarEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if event.UID == "" {
		return fmt.Errorf("event has no UID")
	}

	if len(event.RawData) == 0 {
		return fmt.Errorf("no raw calendar data to store")
	}

	// Read the existing calendar file
	existingData, err := os.ReadFile(s.FilePath)
	if err != nil {
		return fmt.Errorf("reading calendar file: %w", err)
	}

	// Parse the existing calendar
	existingCal, err := ical.NewDecoder(bytes.NewReader(existingData)).Decode()
	if err != nil {
		// If we can't parse it, create a new calendar
		existingCal = ical.NewCalendar()
		existingCal.Props.Set(&ical.Prop{Name: "PRODID", Value: "-//calmailproc//ICalFileStorage//EN"})
		existingCal.Props.Set(&ical.Prop{Name: "VERSION", Value: "2.0"})
	}

	// Parse the new event data
	newCal, err := ical.NewDecoder(bytes.NewReader(event.RawData)).Decode()
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

	// For each new event, check if we should update an existing event or add it
	for _, newEvent := range newEvents {
		uidProp := newEvent.Props.Get("UID")
		if uidProp == nil {
			continue // Skip events without UID
		}

		// Check if this event has a RECURRENCE-ID (it's an exception to a recurring event)
		recurrenceID := newEvent.Props.Get("RECURRENCE-ID")

		// Try to find the existing event with the same UID
		found := false
		for i, component := range existingCal.Children {
			if component.Name != "VEVENT" {
				continue
			}

			existingUID := component.Props.Get("UID")
			if existingUID == nil || existingUID.Value != uidProp.Value {
				continue
			}

			// If both have RECURRENCE-ID, check if they match
			existingRecurrenceID := component.Props.Get("RECURRENCE-ID")
			if recurrenceID != nil && existingRecurrenceID != nil {
				if recurrenceID.Value == existingRecurrenceID.Value {
					// Update this specific occurrence
					existingCal.Children[i] = newEvent
					found = true
					break
				}
			} else if recurrenceID == nil && existingRecurrenceID == nil {
				// Both are master events (no RECURRENCE-ID), update the master
				existingCal.Children[i] = newEvent
				found = true
				break
			}
		}

		// If we didn't find a matching event, add the new event
		if !found {
			existingCal.Children = append(existingCal.Children, newEvent)
		}
	}

	// Write the updated calendar back to the file
	var buf bytes.Buffer
	encoder := ical.NewEncoder(&buf)
	if err := encoder.Encode(existingCal); err != nil {
		return fmt.Errorf("encoding updated calendar: %w", err)
	}

	if err := os.WriteFile(s.FilePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing updated calendar file: %w", err)
	}

	return nil
}

// GetEvent retrieves a calendar event from the iCalendar file by UID
func (s *ICalFileStorage) GetEvent(id string) (*parser.CalendarEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Read the calendar file
	data, err := os.ReadFile(s.FilePath)
	if err != nil {
		return nil, fmt.Errorf("reading calendar file: %w", err)
	}

	// Parse the calendar
	cal, err := ical.NewDecoder(bytes.NewReader(data)).Decode()
	if err != nil {
		return nil, fmt.Errorf("parsing calendar data: %w", err)
	}

	// Find the event with the specified UID
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		uidProp := component.Props.Get("UID")
		if uidProp == nil || uidProp.Value != id {
			continue
		}

		// We found the event, extract its data
		var eventBuf bytes.Buffer
		eventCal := ical.NewCalendar()

		// Copy the relevant properties from the main calendar
		eventCal.Props = cal.Props

		// Add the event component
		eventCal.Children = append(eventCal.Children, component)

		// Also add any related components (like recurring event exceptions)
		for _, otherComponent := range cal.Children {
			if otherComponent.Name != "VEVENT" {
				continue
			}

			otherUID := otherComponent.Props.Get("UID")
			if otherUID == nil || otherUID.Value != id || otherComponent == component {
				continue
			}

			// This is another component with the same UID (likely a recurring event exception)
			eventCal.Children = append(eventCal.Children, otherComponent)
		}

		// Encode the event calendar
		encoder := ical.NewEncoder(&eventBuf)
		if err := encoder.Encode(eventCal); err != nil {
			return nil, fmt.Errorf("encoding event data: %w", err)
		}

		// Create the event object
		event := &parser.CalendarEvent{
			UID:     id,
			RawData: eventBuf.Bytes(),
		}

		// Extract summary if available
		summaryProp := component.Props.Get("SUMMARY")
		if summaryProp != nil {
			event.Summary = summaryProp.Value
		}

		return event, nil
	}

	return nil, fmt.Errorf("event with UID %s not found", id)
}

// ListEvents lists all events in the iCalendar file
func (s *ICalFileStorage) ListEvents() ([]*parser.CalendarEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Read the calendar file
	data, err := os.ReadFile(s.FilePath)
	if err != nil {
		return nil, fmt.Errorf("reading calendar file: %w", err)
	}

	// Parse the calendar
	cal, err := ical.NewDecoder(bytes.NewReader(data)).Decode()
	if err != nil {
		return nil, fmt.Errorf("parsing calendar data: %w", err)
	}

	// Create a map to store unique events by UID
	eventMap := make(map[string]*parser.CalendarEvent)

	// Process all VEVENT components
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		uidProp := component.Props.Get("UID")
		if uidProp == nil {
			continue // Skip events without UID
		}

		uid := uidProp.Value

		// Check if we already have an event with this UID
		if _, exists := eventMap[uid]; !exists {
			// Create a new event for this UID
			event := &parser.CalendarEvent{
				UID: uid,
			}

			// Extract summary if available
			summaryProp := component.Props.Get("SUMMARY")
			if summaryProp != nil {
				event.Summary = summaryProp.Value
			}

			eventMap[uid] = event
		}
	}

	// Convert the map values to a slice
	var events []*parser.CalendarEvent
	for _, event := range eventMap {
		events = append(events, event)
	}

	return events, nil
}

// DeleteEvent deletes a calendar event from the iCalendar file
func (s *ICalFileStorage) DeleteEvent(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Read the calendar file
	data, err := os.ReadFile(s.FilePath)
	if err != nil {
		return fmt.Errorf("reading calendar file: %w", err)
	}

	// Parse the calendar
	cal, err := ical.NewDecoder(bytes.NewReader(data)).Decode()
	if err != nil {
		return fmt.Errorf("parsing calendar data: %w", err)
	}

	// Create a new list of components, excluding the one to delete
	var newChildren []*ical.Component
	deleted := false

	for _, component := range cal.Children {
		if component.Name == "VEVENT" {
			uidProp := component.Props.Get("UID")
			if uidProp != nil && uidProp.Value == id {
				deleted = true
				continue // Skip this component
			}
		}
		newChildren = append(newChildren, component)
	}

	if !deleted {
		return fmt.Errorf("event with UID %s not found", id)
	}

	// Update the calendar with the new component list
	cal.Children = newChildren

	// Write the updated calendar back to the file
	var buf bytes.Buffer
	encoder := ical.NewEncoder(&buf)
	if err := encoder.Encode(cal); err != nil {
		return fmt.Errorf("encoding updated calendar: %w", err)
	}

	if err := os.WriteFile(s.FilePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing updated calendar file: %w", err)
	}

	return nil
}
