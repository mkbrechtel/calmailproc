package vdir

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mkbrechtel/calmailproc/manager"
	"github.com/mkbrechtel/calmailproc/parser/ical"
)

// VDirStorage implements the storage.Storage interface using the vdir format
type VDirStorage struct {
	BasePath        string
	calendarManager manager.Calendar
}

// NewVDirStorage creates a new VDirStorage with the given base path
func NewVDirStorage(basePath string) (*VDirStorage, error) {
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("creating storage directory: %w", err)
	}

	return &VDirStorage{
		BasePath:        basePath,
		calendarManager: manager.NewDefaultManager(),
	}, nil
}

// StoreEvent stores a calendar event in the vdir format
func (s *VDirStorage) StoreEvent(event *ical.Event) error {
	if event.UID == "" {
		return fmt.Errorf("event has no UID")
	}

	if len(event.RawData) == 0 {
		return fmt.Errorf("no raw calendar data to store")
	}

	// Create the event file with .ics extension
	filename := fmt.Sprintf("%s.ics", event.UID)
	filePath := filepath.Join(s.BasePath, filename)

	// Parse the iCalendar data using panic recovery
	cal, err := ical.DecodeCalendar(event.RawData)
	
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
	existingCal, err := ical.DecodeCalendar(existingData)
	
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

	// Load existing event if possible (for METHOD:REPLY handling)
	var existingEvent *ical.Event
	if event.Method == "REPLY" {
		existingData, err := os.ReadFile(filePath)
		if err == nil {
			existingEvent = &ical.Event{
				UID:     event.UID,
				RawData: existingData,
			}
			
			// Try to update attendee status
			if err := s.calendarManager.UpdateAttendeeStatus(event, existingEvent); err != nil {
				// Log but continue with regular processing
				fmt.Printf("Warning: Failed to update attendee status: %v\n", err)
			} else {
				// Successfully updated attendee status, write and return
				if err := os.WriteFile(filePath, existingEvent.RawData, 0644); err != nil {
					return fmt.Errorf("writing updated calendar file: %w", err)
				}
				return nil
			}
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
		
		// Write the updated calendar back to the file
		calBytes, err := ical.EncodeCalendar(updatedCal)
		if err != nil {
			return fmt.Errorf("encoding updated calendar: %w", err)
		}
		
		if err := os.WriteFile(filePath, calBytes, 0644); err != nil {
			return fmt.Errorf("writing updated calendar file: %w", err)
		}
		
		return nil
	}

	// This is a new master event or a complete update
	// Just write the new data, overwriting the old file
	if err := os.WriteFile(filePath, event.RawData, 0644); err != nil {
		return fmt.Errorf("writing calendar file: %w", err)
	}

	return nil
}

// GetEvent retrieves a calendar event from the vdir storage
func (s *VDirStorage) GetEvent(id string) (*ical.Event, error) {
	// Find the event file
	filename := fmt.Sprintf("%s.ics", id)
	filePath := filepath.Join(s.BasePath, filename)

	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading event file: %w", err)
	}

	// Create a basic event with the raw data
	event := &ical.Event{
		UID:     id,
		RawData: data,
	}

	// Extract minimal information for display purposes
	cal, parseErr := ical.DecodeCalendar(data)
	if parseErr != nil {
		// Set default summary for events that can't be parsed
		event.Summary = "Unparseable Calendar Event"
	}
	
	if parseErr == nil { // Ignore parsing errors, we already have the raw data
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
func (s *VDirStorage) ListEvents() ([]*ical.Event, error) {
	// List all .ics files in the directory
	entries, err := os.ReadDir(s.BasePath)
	if err != nil {
		return nil, fmt.Errorf("reading storage directory: %w", err)
	}

	var events []*ical.Event
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