package vdir

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emersion/go-ical"
	"github.com/mkbrechtel/calmailproc/parser"
)

// VDirStorage implements the storage.Storage interface using the vdir format
type VDirStorage struct {
	BasePath string
}

// NewVDirStorage creates a new VDirStorage with the given base path
func NewVDirStorage(basePath string) (*VDirStorage, error) {
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("creating storage directory: %w", err)
	}

	return &VDirStorage{
		BasePath: basePath,
	}, nil
}

// StoreEvent stores a calendar event in the vdir format
func (s *VDirStorage) StoreEvent(event *parser.CalendarEvent) error {
	if event.UID == "" {
		return fmt.Errorf("event has no UID")
	}

	if len(event.RawData) == 0 {
		return fmt.Errorf("no raw calendar data to store")
	}

	// Create the event file with .ics extension
	filename := fmt.Sprintf("%s.ics", event.UID)
	filePath := filepath.Join(s.BasePath, filename)

	// Parse the iCalendar data
	cal, err := ical.NewDecoder(bytes.NewReader(event.RawData)).Decode()
	if err != nil {
		// If we can't parse it, just store the raw data as before
		if err := os.WriteFile(filePath, event.RawData, 0644); err != nil {
			return fmt.Errorf("writing calendar file: %w", err)
		}
		return nil
	}

	// Extract the METHOD if present
	methodValue := ""
	methodProp := cal.Props.Get("METHOD")
	if methodProp != nil {
		methodValue = methodProp.Value
	}

	// Check if file already exists - could be a recurring event update
	existingData, err := os.ReadFile(filePath)
	if err != nil {
		// File doesn't exist yet, create new
		if err := os.WriteFile(filePath, event.RawData, 0644); err != nil {
			return fmt.Errorf("writing calendar file: %w", err)
		}
		return nil
	}

	// File exists, check if we need to merge this event with existing data
	existingCal, err := ical.NewDecoder(bytes.NewReader(existingData)).Decode()
	if err != nil {
		// Can't parse existing, overwrite with new data
		if err := os.WriteFile(filePath, event.RawData, 0644); err != nil {
			return fmt.Errorf("writing calendar file: %w", err)
		}
		return nil
	}

	// Extract the current event from the new data
	var newEvent *ical.Component
	for _, component := range cal.Children {
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
		return s.handleRecurringEventUpdate(existingCal, newEvent, methodValue, filePath)
	}

	// This is a new master event or a complete update
	// Just write the new data, overwriting the old file
	if err := os.WriteFile(filePath, event.RawData, 0644); err != nil {
		return fmt.Errorf("writing calendar file: %w", err)
	}

	return nil
}

// handleRecurringEventUpdate merges a recurring event update into the existing calendar file
func (s *VDirStorage) handleRecurringEventUpdate(existingCal *ical.Calendar, newEvent *ical.Component, methodValue string, filePath string) error {
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

	// Write the updated calendar back to the file
	var buf bytes.Buffer
	encoder := ical.NewEncoder(&buf)
	if err := encoder.Encode(existingCal); err != nil {
		return fmt.Errorf("encoding updated calendar: %w", err)
	}

	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing updated calendar file: %w", err)
	}

	return nil
}

// GetEvent retrieves a calendar event from the vdir storage
func (s *VDirStorage) GetEvent(id string) (*parser.CalendarEvent, error) {
	// Find the event file
	filename := fmt.Sprintf("%s.ics", id)
	filePath := filepath.Join(s.BasePath, filename)

	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading event file: %w", err)
	}

	// Create a basic event with the raw data
	event := &parser.CalendarEvent{
		UID:     id,
		RawData: data,
	}

	// Extract minimal information for display purposes
	cal, err := ical.NewDecoder(bytes.NewReader(data)).Decode()
	if err == nil { // Ignore parsing errors, we already have the raw data
		for _, component := range cal.Children {
			if component.Name != "VEVENT" {
				continue
			}

			// Extract Summary if available
			summaryProp := component.Props.Get("SUMMARY")
			if summaryProp != nil {
				event.Summary = summaryProp.Value
			}
			break
		}
	}

	return event, nil
}

// ListEvents lists all events in the vdir storage
func (s *VDirStorage) ListEvents() ([]*parser.CalendarEvent, error) {
	// List all .ics files in the directory
	entries, err := os.ReadDir(s.BasePath)
	if err != nil {
		return nil, fmt.Errorf("reading storage directory: %w", err)
	}

	var events []*parser.CalendarEvent
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ics") {
			continue
		}

		// Extract the UID from the filename
		uid := strings.TrimSuffix(entry.Name(), ".ics")

		// Get the event
		event, err := s.GetEvent(uid)
		if err != nil {
			continue
		}

		events = append(events, event)
	}

	return events, nil
}

// DeleteEvent deletes a calendar event from the vdir storage
func (s *VDirStorage) DeleteEvent(id string) error {
	// Find the event file
	filename := fmt.Sprintf("%s.ics", id)
	filePath := filepath.Join(s.BasePath, filename)

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("deleting event file: %w", err)
	}

	return nil
}
