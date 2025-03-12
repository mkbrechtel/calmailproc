package manager

import (
	"fmt"

	goical "github.com/emersion/go-ical"
	"github.com/mkbrechtel/calmailproc/parser/ical"
)

// Calendar defines the interface for managing calendar events
type Calendar interface {
	// UpdateAttendeeStatus updates attendee status for a given event
	// This is typically used when processing METHOD:REPLY events
	UpdateAttendeeStatus(event *ical.Event, storageEvent *ical.Event) error

	// HandleRecurringEventUpdate handles updates to recurring events (with RECURRENCE-ID)
	// This was previously duplicated across storage implementations
	HandleRecurringEventUpdate(existingCal *goical.Calendar, newEvent *goical.Component, methodValue string) (*goical.Calendar, error)
}

// DefaultManager implements the Calendar interface with standard calendar management logic
type DefaultManager struct{}

// NewDefaultManager creates a new DefaultManager
func NewDefaultManager() *DefaultManager {
	return &DefaultManager{}
}

// UpdateAttendeeStatus updates attendee status based on the event's METHOD
// Primarily handles METHOD:REPLY to update attendee participation status
func (m *DefaultManager) UpdateAttendeeStatus(event *ical.Event, storageEvent *ical.Event) error {
	// Skip non-REPLY events
	if event.Method != "REPLY" {
		return nil
	}

	// Parse the new event data
	newCal, err := ical.DecodeCalendar(event.RawData)
	if err != nil {
		return fmt.Errorf("parsing event data: %w", err)
	}

	// Parse the existing event data
	existingCal, err := ical.DecodeCalendar(storageEvent.RawData)
	if err != nil {
		return fmt.Errorf("parsing existing event data: %w", err)
	}

	// Extract the replying attendee from the new data
	var replyEvent *goical.Component
	for _, component := range newCal.Children {
		if component.Name == "VEVENT" {
			replyEvent = component
			break
		}
	}
	if replyEvent == nil {
		return fmt.Errorf("no VEVENT component found in reply")
	}

	// Find attendee in the reply
	attendeeProp := replyEvent.Props.Get("ATTENDEE")
	if attendeeProp == nil {
		return fmt.Errorf("no ATTENDEE property in reply")
	}

	attendeeEmail := attendeeProp.Value
	
	// Get the PARTSTAT (participation status) from the reply
	attendeeStatus := ""
	if partstat := attendeeProp.Params.Get("PARTSTAT"); partstat != "" {
		attendeeStatus = partstat
	} else {
		return fmt.Errorf("no PARTSTAT found in reply")
	}

	// Update the attendee status in the existing event
	updated := false
	for _, component := range existingCal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		// Check if this is the same instance (RECURRENCE-ID matching if present)
		recurrenceIDMatch := matchesRecurrenceID(replyEvent, component)
		if !recurrenceIDMatch {
			continue
		}

		// Find and update the matching attendee
		attendeeProps := component.Props["ATTENDEE"]
		for i, prop := range attendeeProps {
			if prop.Value != attendeeEmail {
				continue
			}

			// Update the PARTSTAT parameter
			prop.Params.Set("PARTSTAT", attendeeStatus)
			attendeeProps[i] = prop
			updated = true
			break
		}
		
		if updated {
			component.Props["ATTENDEE"] = attendeeProps
			break
		}
	}

	if !updated {
		return fmt.Errorf("attendee %s not found in event", attendeeEmail)
	}

	// Encode the updated calendar back to bytes
	calBytes, err := ical.EncodeCalendar(existingCal)
	if err != nil {
		return fmt.Errorf("encoding updated calendar: %w", err)
	}

	// Update the storage event
	storageEvent.RawData = calBytes
	return nil
}

// HandleRecurringEventUpdate merges a recurring event update into the existing calendar
// This function consolidates the duplicate code from various storage implementations
func (m *DefaultManager) HandleRecurringEventUpdate(existingCal *goical.Calendar, newEvent *goical.Component, methodValue string) (*goical.Calendar, error) {
	recurrenceID := newEvent.Props.Get("RECURRENCE-ID")
	if recurrenceID == nil {
		return nil, fmt.Errorf("missing RECURRENCE-ID in event update")
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
				component.Props.Set(&goical.Prop{Name: "STATUS", Value: "CANCELLED"})
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
			newEvent.Props.Set(&goical.Prop{Name: "STATUS", Value: "CANCELLED"})
			existingCal.Children = append(existingCal.Children, newEvent)
		} else {
			// Add the new occurrence
			existingCal.Children = append(existingCal.Children, newEvent)
		}
	}

	return existingCal, nil
}

// matchesRecurrenceID checks if two events refer to the same instance
// by comparing their RECURRENCE-ID properties
func matchesRecurrenceID(event1, event2 *goical.Component) bool {
	recurrenceID1 := event1.Props.Get("RECURRENCE-ID")
	recurrenceID2 := event2.Props.Get("RECURRENCE-ID")

	// If both have RECURRENCE-ID, they must match
	if recurrenceID1 != nil && recurrenceID2 != nil {
		return recurrenceID1.Value == recurrenceID2.Value
	}
	
	// If neither has RECURRENCE-ID, they refer to the master event
	if recurrenceID1 == nil && recurrenceID2 == nil {
		return true
	}
	
	// One has RECURRENCE-ID and the other doesn't, so they're different instances
	return false
}