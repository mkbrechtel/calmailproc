package icalfile

import (
	"fmt"
	"os"
	"sync"

	"github.com/mkbrechtel/calmailproc/manager"
	"github.com/mkbrechtel/calmailproc/parser/ical"
)

// ICalFileStorage implements the storage.Storage interface using a single iCalendar file
type ICalFileStorage struct {
	FilePath        string
	mu              sync.Mutex // Mutex to protect concurrent file access
	calendarManager manager.Calendar
}

// NewICalFileStorage creates a new ICalFileStorage with the given file path
func NewICalFileStorage(filePath string) (*ICalFileStorage, error) {
	storage := &ICalFileStorage{
		FilePath:        filePath,
		calendarManager: manager.NewDefaultManager(),
	}

	// Create an empty calendar file if it doesn't exist
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Create a minimal valid iCalendar file
		cal := ical.NewCalendar()
		initialContent, err := ical.EncodeCalendar(cal)
		if err != nil {
			return nil, fmt.Errorf("creating initial calendar content: %w", err)
		}
		if err := os.WriteFile(filePath, initialContent, 0644); err != nil {
			return nil, fmt.Errorf("creating initial calendar file: %w", err)
		}
	}

	return storage, nil
}

// StoreEvent stores a calendar event in the single iCalendar file
func (s *ICalFileStorage) StoreEvent(event *ical.Event) error {
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
	existingCal, err := ical.DecodeCalendar(existingData)
	if err != nil {
		// If we can't parse it, create a new calendar
		existingCal = ical.NewCalendar()
	}

	// Parse the new event data
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
	
	// Check if this is a METHOD:REPLY for attendee status updates
	methodProp := newCal.Props.Get("METHOD")
	if methodProp != nil && methodProp.Value == "REPLY" {
		// Find the existing event with the same UID to update attendee status
		for _, newEvent := range newEvents {
			uidProp := newEvent.Props.Get("UID")
			if uidProp == nil {
				continue // Skip events without UID
			}
			
			// Create a temporary event to pass to the attendee manager
			replyEvent := &ical.Event{
				UID:     uidProp.Value,
				Method:  "REPLY",
				RawData: event.RawData,
			}
			
			// Look for the matching event in the existing calendar
			existingEvent := findMatchingEvent(existingCal, uidProp.Value, newEvent)
			if existingEvent != nil {
				// Create a calendar with just this event
				tempCal := ical.NewCalendar()
				tempCal.Children = append(tempCal.Children, existingEvent)
				
				// Convert to bytes for the storage event
				calBytes, err := ical.EncodeCalendar(tempCal)
				if err != nil {
					return fmt.Errorf("encoding existing event for attendee update: %w", err)
				}
				
				// Create a storage event
				storageEvent := &ical.Event{
					UID:     uidProp.Value,
					RawData: calBytes,
				}
				
				// Try to update attendee status
				if err := s.calendarManager.UpdateAttendeeStatus(replyEvent, storageEvent); err == nil {
					// Successfully updated, replace the existing event with updated one
					updatedCal, err := ical.DecodeCalendar(storageEvent.RawData)
					if err != nil {
						return fmt.Errorf("parsing updated event data: %w", err)
					}
					
					// Replace the existing event with the updated one
					for i, component := range existingCal.Children {
						if component == existingEvent {
							existingCal.Children[i] = updatedCal.Children[0]
							break
						}
					}
				}
			}
		}
	}

	// For each new event, check if we should update an existing event or add it
	for _, newEvent := range newEvents {
		uidProp := newEvent.Props.Get("UID")
		if uidProp == nil {
			continue // Skip events without UID
		}

		// Check if this event has a RECURRENCE-ID (it's an exception to a recurring event)
		recurrenceID := newEvent.Props.Get("RECURRENCE-ID")
		
		// Get method value if present
		methodValue := ""
		if methodProp != nil {
			methodValue = methodProp.Value
		}

		// If this is a recurring instance update, use the attendee manager
		if recurrenceID != nil {
			// Find matching UID events in the calendar
			var masterEventFound bool
			for _, component := range existingCal.Children {
				if component.Name != "VEVENT" {
					continue
				}
				
				existingUID := component.Props.Get("UID")
				if existingUID == nil || existingUID.Value != uidProp.Value {
					continue
				}
				
				masterEventFound = true
				// We have at least one event with this UID
				break
			}
			
			if masterEventFound {
				// Filter calendar to only include events with this UID
				tempCal := ical.NewCalendar()
				tempCal.Props = existingCal.Props
				
				for _, component := range existingCal.Children {
					if component.Name != "VEVENT" {
						continue
					}
					
					existingUID := component.Props.Get("UID")
					if existingUID != nil && existingUID.Value == uidProp.Value {
						tempCal.Children = append(tempCal.Children, component)
					}
				}
				
				// Handle the recurring event update
				updatedCal, err := s.calendarManager.HandleRecurringEventUpdate(tempCal, newEvent, methodValue)
				if err != nil {
					return fmt.Errorf("handling recurring event update: %w", err)
				}
				
				// Remove all events with this UID from the main calendar
				var newChildren []*ical.Component
				for _, component := range existingCal.Children {
					if component.Name != "VEVENT" {
						newChildren = append(newChildren, component)
						continue
					}
					
					existingUID := component.Props.Get("UID")
					if existingUID == nil || existingUID.Value != uidProp.Value {
						newChildren = append(newChildren, component)
					}
				}
				
				// Add all events from the updated calendar
				existingCal.Children = append(newChildren, updatedCal.Children...)
				continue
			}
		}

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
	calBytes, err := ical.EncodeCalendar(existingCal)
	if err != nil {
		return fmt.Errorf("encoding updated calendar: %w", err)
	}

	if err := os.WriteFile(s.FilePath, calBytes, 0644); err != nil {
		return fmt.Errorf("writing updated calendar file: %w", err)
	}

	return nil
}

// GetEvent retrieves a calendar event from the iCalendar file by UID
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
		eventBytes, err := ical.EncodeCalendar(eventCal)
		if err != nil {
			return nil, fmt.Errorf("encoding event data: %w", err)
		}

		// Create the event object
		event := &ical.Event{
			UID:     id,
			RawData: eventBytes,
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
		return nil, fmt.Errorf("parsing calendar data: %w", err)
	}

	// Create a map to store unique events by UID
	eventMap := make(map[string]*ical.Event)

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
			event := &ical.Event{
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
	var events []*ical.Event
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
	cal, err := ical.DecodeCalendar(data)
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
	calBytes, err := ical.EncodeCalendar(cal)
	if err != nil {
		return fmt.Errorf("encoding updated calendar: %w", err)
	}

	if err := os.WriteFile(s.FilePath, calBytes, 0644); err != nil {
		return fmt.Errorf("writing updated calendar file: %w", err)
	}

	return nil
}
// findMatchingEvent finds an event in a calendar that matches the given UID and
// has the same RECURRENCE-ID as the new event (if any)
func findMatchingEvent(cal *ical.Calendar, uid string, newEvent *ical.Component) *ical.Component {
	// Get the RECURRENCE-ID from the new event if it exists
	recurrenceID := newEvent.Props.Get("RECURRENCE-ID")
	
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}
		
		// Check if the UID matches
		uidProp := component.Props.Get("UID")
		if uidProp == nil || uidProp.Value != uid {
			continue
		}
		
		// If the new event has a RECURRENCE-ID, we need to find a matching occurrence
		if recurrenceID != nil {
			existingRecurrenceID := component.Props.Get("RECURRENCE-ID")
			if existingRecurrenceID == nil || existingRecurrenceID.Value != recurrenceID.Value {
				continue // Not the same occurrence
			}
		} else {
			// If the new event doesn't have a RECURRENCE-ID, we want the master event
			existingRecurrenceID := component.Props.Get("RECURRENCE-ID")
			if existingRecurrenceID != nil {
				continue // This is a specific occurrence, not the master
			}
		}
		
		// Found a matching event
		return component
	}
	
	// No matching event found
	return nil
}
