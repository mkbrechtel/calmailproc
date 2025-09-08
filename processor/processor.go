package processor

import (
	"fmt"
	"io"
	"time"

	goical "github.com/emersion/go-ical"
	"github.com/mkbrechtel/calmailproc/parser/email"
	"github.com/mkbrechtel/calmailproc/parser/ical"
	"github.com/mkbrechtel/calmailproc/storage"
)

// Processor handles processing emails with calendar events
type Processor struct {
	Storage        storage.Storage
	ProcessReplies bool // Whether to process METHOD:REPLY to update attendee status
}

func NewProcessor(storage storage.Storage, processReplies bool) *Processor {
	return &Processor{
		Storage:        storage,
		ProcessReplies: processReplies,
	}
}

func (p *Processor) ProcessEmail(r io.Reader) (string, error) {
	parsedEmail, err := email.Parse(r)
	if err != nil {
		return "E-Mail parsing error", fmt.Errorf("parsing email: %w", err)
	}

	// Process the calendar event if one was found (always store if it has a valid UID)
	if parsedEmail.HasCalendar && parsedEmail.Event.UID != "" {
		// Validate the UID before processing
		if err := ical.ValidateUID(parsedEmail.Event.UID); err != nil {
			return fmt.Sprintf("Invalid UID for calendar event: %v", err), err
		}
		// Check if this is a METHOD:REQUEST or METHOD:CANCEL
		if parsedEmail.Event.Method == "REQUEST" {
			return p.processEventRequest(parsedEmail)
		} else if parsedEmail.Event.Method == "CANCEL" {
			return p.processEventCancelation(parsedEmail)
		} else if parsedEmail.Event.Method == "REPLY" {
			return p.processEventReply(parsedEmail)
		} else {
			return p.processEvent(parsedEmail)
		}
	} else {
		return "Processed E-Mail without calendar event", nil
	}
}

func (p *Processor) processEvent(parsedEmail *email.Email) (string, error) {
	// First, validate the event by testing decode and encode
	if err := ical.ValidateEvent(parsedEmail.Event.RawData); err != nil {
		return fmt.Sprintf("Invalid calendar data for event with UID %s", parsedEmail.Event.UID),
			fmt.Errorf("validation error for event %s: %w", parsedEmail.Event.UID, err)
	}

	// Check if this is a recurring instance update (has RECURRENCE-ID)
	// We handle instance updates differently from parent event updates
	isInstanceUpdate := parsedEmail.Event.IsRecurringUpdate()

	// Check for existing event with the same UID
	existingEvent, err := p.Storage.GetEvent(parsedEmail.Event.UID)
	if err == nil && existingEvent != nil {
		// If this is an instance update, we always process it regardless of parent sequence
		if isInstanceUpdate {
			// Handle recurring instance update
			updatedEvent, err := p.handleRecurringEvent(existingEvent, parsedEmail.Event)
			if err != nil {
				return "Error handling recurring instance", fmt.Errorf("handling recurring instance: %w", err)
			}

			// Validate the updated event
			if err := ical.ValidateEvent(updatedEvent.RawData); err != nil {
				return fmt.Sprintf("Invalid calendar data after instance update for event with UID %s", updatedEvent.UID),
					fmt.Errorf("validation error after instance update for event %s: %w", updatedEvent.UID, err)
			}

			// Prepare and store the updated event
			preparedEvent, err := prepareEventForStorage(updatedEvent)
			if err != nil {
				return "Error preparing event for storage", fmt.Errorf("preparing event: %w", err)
			}
			if err := p.Storage.StoreEvent(preparedEvent); err != nil {
				return "Error storing updated event", fmt.Errorf("storing updated event: %w", err)
			}

			return fmt.Sprintf("Updated recurring event instance with UID %s", parsedEmail.Event.UID), nil
		} else {
			// This is a parent event update, not an instance update
			// Check if the existing event is also a parent or an instance
			existingIsInstance := existingEvent.IsRecurringUpdate()
			
			// If existing event is an instance but new event is a parent,
			// we should handle them independently
			if existingIsInstance && !isInstanceUpdate {
				// Update the parent while preserving instances
				updatedEvent, err := p.handleParentEventUpdate(existingEvent, parsedEmail.Event)
				if err != nil {
					return "Error handling parent event update", fmt.Errorf("handling parent event update: %w", err)
				}

				// Validate the updated event
				if err := ical.ValidateEvent(updatedEvent.RawData); err != nil {
					return fmt.Sprintf("Invalid calendar data after parent update for event with UID %s", updatedEvent.UID),
						fmt.Errorf("validation error after parent update for event %s: %w", updatedEvent.UID, err)
				}

				// Prepare and store the updated event
				preparedEvent, err := prepareEventForStorage(updatedEvent)
				if err != nil {
					return "Error preparing event for storage", fmt.Errorf("preparing event: %w", err)
				}
				if err := p.Storage.StoreEvent(preparedEvent); err != nil {
					return "Error storing updated event", fmt.Errorf("storing updated event: %w", err)
				}

				return fmt.Sprintf("Updated parent event while preserving instances with UID %s", parsedEmail.Event.UID), nil
			}
			
			// Regular parent-to-parent comparison
			comparison, err := ical.CompareEvents(parsedEmail.Event, existingEvent)
			if err != nil {
				return "Error comparing events", fmt.Errorf("comparing events: %w", err)
			}
			
			// Only update if the new event is newer or equal to the existing one
			if comparison == ical.SecondEventNewer {
				return fmt.Sprintf("Not processing older event (sequence: %d vs %d, DTSTAMP comparison) with UID %s",
					parsedEmail.Event.Sequence, existingEvent.Sequence,
					parsedEmail.Event.UID), nil
			} else {
				// This is a parent event update that should be processed
				// We need to handle it while preserving any existing instances
				updatedEvent, err := p.handleParentEventUpdate(existingEvent, parsedEmail.Event)
				if err != nil {
					return "Error handling parent event update", fmt.Errorf("handling parent event update: %w", err)
				}

				// Validate the updated event
				if err := ical.ValidateEvent(updatedEvent.RawData); err != nil {
					return fmt.Sprintf("Invalid calendar data after parent update for event with UID %s", updatedEvent.UID),
						fmt.Errorf("validation error after parent update for event %s: %w", updatedEvent.UID, err)
				}

				// Prepare and store the updated event
				preparedEvent, err := prepareEventForStorage(updatedEvent)
				if err != nil {
					return "Error preparing event for storage", fmt.Errorf("preparing event: %w", err)
				}
				if err := p.Storage.StoreEvent(preparedEvent); err != nil {
					return "Error storing updated event", fmt.Errorf("storing updated event: %w", err)
				}

				return fmt.Sprintf("Updated event with UID %s, new sequence: %d",
					parsedEmail.Event.UID, parsedEmail.Event.Sequence), nil
			}
		}
	} else {
		// No existing event found, prepare and store the new one
		preparedEvent, err := prepareEventForStorage(parsedEmail.Event)
		if err != nil {
			return "Error preparing event for storage", fmt.Errorf("preparing event: %w", err)
		}
		if err := p.Storage.StoreEvent(preparedEvent); err != nil {
			return "Error storing new event", fmt.Errorf("storing event: %w", err)
		}

		return fmt.Sprintf("Stored new event with UID %s", parsedEmail.Event.UID), nil
	}
}

// processEventRequest handles calendar events with METHOD:REQUEST
func (p *Processor) processEventRequest(parsedEmail *email.Email) (string, error) {
	return p.processEvent(parsedEmail)
}

// processEventCancelation handles calendar events with METHOD:CANCEL
func (p *Processor) processEventCancelation(parsedEmail *email.Email) (string, error) {
	return p.processEvent(parsedEmail)
}

// processEventReply handles calendar events with METHOD:REPLY
func (p *Processor) processEventReply(parsedEmail *email.Email) (string, error) {
	if !p.ProcessReplies {
		// Skip storing REPLY events when ProcessReplies is false
		return "Ignoring calendar REPLY method as configured", nil
	}

	// First, validate the event
	if err := ical.ValidateEvent(parsedEmail.Event.RawData); err != nil {
		return fmt.Sprintf("Invalid calendar data for event reply with UID %s", parsedEmail.Event.UID),
			fmt.Errorf("validation error for event reply %s: %w", parsedEmail.Event.UID, err)
	}

	// Try to find the existing event to update attendee status
	existingEvent, err := p.Storage.GetEvent(parsedEmail.Event.UID)
	if err == nil && existingEvent != nil {
		// Process the reply to update attendee status
		if err := p.updateAttendeeStatus(parsedEmail.Event, existingEvent); err != nil {
			fmt.Printf("Warning: Failed to update attendee status: %v\n", err)

			// If attendee update fails, prepare and store the event normally
			preparedEvent, err := prepareEventForStorage(parsedEmail.Event)
			if err != nil {
				return "Error preparing event for storage", fmt.Errorf("preparing event: %w", err)
			}
			if err := p.Storage.StoreEvent(preparedEvent); err != nil {
				return "Error storing reply event", fmt.Errorf("storing event: %w", err)
			}

			return fmt.Sprintf("Stored reply event with UID %s (attendee update failed)",
				parsedEmail.Event.UID), nil
		} else {
			// Validate the updated event before storing
			if err := ical.ValidateEvent(existingEvent.RawData); err != nil {
				return fmt.Sprintf("Invalid calendar data after attendee update for event with UID %s", existingEvent.UID),
					fmt.Errorf("validation error after attendee update for event %s: %w", existingEvent.UID, err)
			}

			// Prepare and store the updated event
			preparedEvent, err := prepareEventForStorage(existingEvent)
			if err != nil {
				return "Error preparing event for storage", fmt.Errorf("preparing event: %w", err)
			}
			if err := p.Storage.StoreEvent(preparedEvent); err != nil {
				return "Error storing updated event with attendee status", fmt.Errorf("storing updated event: %w", err)
			}

			return fmt.Sprintf("Updated attendee status for event with UID %s",
				parsedEmail.Event.UID), nil
		}
	} else {
		// No existing event found, prepare and store the new one
		preparedEvent, err := prepareEventForStorage(parsedEmail.Event)
		if err != nil {
			return "Error preparing event for storage", fmt.Errorf("preparing event: %w", err)
		}
		if err := p.Storage.StoreEvent(preparedEvent); err != nil {
			return "Error storing new reply event", fmt.Errorf("storing event: %w", err)
		}

		return fmt.Sprintf("Stored new reply event with UID %s", parsedEmail.Event.UID), nil
	}
}

// handleRecurringEvent merges a recurring event update with the existing event
func (p *Processor) handleRecurringEvent(existingEvent, newEvent *ical.Event) (*ical.Event, error) {
	// Parse the new event data
	newCal, err := ical.DecodeCalendar(newEvent.RawData)
	if err != nil {
		return nil, fmt.Errorf("parsing new event data: %w", err)
	}

	// Parse the existing event data
	existingCal, err := ical.DecodeCalendar(existingEvent.RawData)
	if err != nil {
		return nil, fmt.Errorf("parsing existing event data: %w", err)
	}

	// Find the VEVENT component with RECURRENCE-ID
	var recurrenceEvent *goical.Component
	for _, component := range newCal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		if component.Props.Get("RECURRENCE-ID") != nil {
			recurrenceEvent = component
			break
		}
	}

	if recurrenceEvent == nil {
		return nil, fmt.Errorf("no recurring event found in update")
	}

	// Ensure the component has a DTSTAMP (required by the iCalendar spec)
	if recurrenceEvent.Props.Get("DTSTAMP") == nil {
		// If no DTSTAMP, add one with the current time
		now := time.Now().UTC().Format("20060102T150405Z")
		recurrenceEvent.Props.Set(&goical.Prop{Name: "DTSTAMP", Value: now})
	}

	// Handle the recurrence update
	recurrenceID := recurrenceEvent.Props.Get("RECURRENCE-ID")
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
			if newEvent.Method == "CANCEL" {
				// For cancellations, we update the status to CANCELLED
				component.Props.Set(&goical.Prop{Name: "STATUS", Value: "CANCELLED"})
			} else {
				// Replace the existing occurrence with the new one
				existingCal.Children[i] = recurrenceEvent
			}
			break
		}
	}

	// If we didn't find an existing occurrence with this RECURRENCE-ID, add it
	if !foundExisting {
		if newEvent.Method == "CANCEL" {
			// For cancellations of instances we haven't seen before, create a new component with STATUS:CANCELLED
			recurrenceEvent.Props.Set(&goical.Prop{Name: "STATUS", Value: "CANCELLED"})
			existingCal.Children = append(existingCal.Children, recurrenceEvent)
		} else {
			// Add the new occurrence
			existingCal.Children = append(existingCal.Children, recurrenceEvent)
		}
	}

	// Ensure all components have DTSTAMP
	for _, component := range existingCal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		if component.Props.Get("DTSTAMP") == nil {
			now := time.Now().UTC().Format("20060102T150405Z")
			component.Props.Set(&goical.Prop{Name: "DTSTAMP", Value: now})
		}
	}

	// Encode the updated calendar back to bytes
	calBytes, err := ical.EncodeCalendar(existingCal)
	if err != nil {
		return nil, fmt.Errorf("encoding updated calendar: %w", err)
	}

	// Create a new event with the updated data
	updatedEvent := &ical.Event{
		UID:      existingEvent.UID,
		RawData:  calBytes,
		Summary:  existingEvent.Summary,
		Method:   existingEvent.Method,
		Sequence: existingEvent.Sequence,
	}

	return updatedEvent, nil
}

// updateAttendeeStatus updates attendee status based on the event's METHOD
// Primarily handles METHOD:REPLY to update attendee participation status
func (p *Processor) updateAttendeeStatus(event *ical.Event, storageEvent *ical.Event) error {
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
		if !matchesRecurrenceID(replyEvent, component) {
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

// handleParentEventUpdate processes a parent event update while preserving any
// existing instance exceptions
func (p *Processor) handleParentEventUpdate(existingEvent, newEvent *ical.Event) (*ical.Event, error) {
	// Parse the new parent event data
	newCal, err := ical.DecodeCalendar(newEvent.RawData)
	if err != nil {
		return nil, fmt.Errorf("parsing new parent event data: %w", err)
	}

	// Parse the existing event data
	existingCal, err := ical.DecodeCalendar(existingEvent.RawData)
	if err != nil {
		return nil, fmt.Errorf("parsing existing event data: %w", err)
	}

	// Find the parent/master event component in the new calendar
	var newParentComponent *goical.Component
	for _, component := range newCal.Children {
		if component.Name != "VEVENT" {
			continue
		}
		
		// Parent events don't have RECURRENCE-ID
		if component.Props.Get("RECURRENCE-ID") == nil {
			newParentComponent = component
			break
		}
	}

	if newParentComponent == nil {
		return nil, fmt.Errorf("no parent event component found in update")
	}

	// Extract all existing instance exceptions
	var existingInstanceComponents []*goical.Component

	for _, component := range existingCal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		// Separate parent event from instance exceptions
		if component.Props.Get("RECURRENCE-ID") != nil {
			// This is an instance exception, preserve it
			existingInstanceComponents = append(existingInstanceComponents, component)
		}
	}

	// Create a new calendar with updated parent event and preserved instances
	updatedCal := goical.NewCalendar()
	
	// Copy the calendar properties
	for name, props := range newCal.Props {
		updatedCal.Props[name] = props
	}

	// Add the updated parent component
	updatedCal.Children = append(updatedCal.Children, newParentComponent)

	// Add all preserved instance exceptions
	for _, instance := range existingInstanceComponents {
		updatedCal.Children = append(updatedCal.Children, instance)
	}

	// Encode the updated calendar back to bytes
	calBytes, err := ical.EncodeCalendar(updatedCal)
	if err != nil {
		return nil, fmt.Errorf("encoding updated calendar: %w", err)
	}

	// Create a new event with the updated data
	updatedEvent := &ical.Event{
		UID:      existingEvent.UID,
		RawData:  calBytes,
		Summary:  newEvent.Summary,
		Method:   newEvent.Method,
		Sequence: newEvent.Sequence, // Use the new sequence number from the parent update
	}

	return updatedEvent, nil
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

// existingEventSequence gets the sequence number of an existing event
// Returns -1 if the event doesn't exist or there's an error
func existingEventSequence(store storage.Storage, uid string) int {
	existingEvent, err := store.GetEvent(uid)
	if err == nil && existingEvent != nil {
		return existingEvent.Sequence
	}
	return -1 // Return -1 if event doesn't exist or there's an error
}

// prepareEventForStorage removes the METHOD property from an event before storing
// This ensures the stored calendar data doesn't contain METHOD which is only for transport
func prepareEventForStorage(event *ical.Event) (*ical.Event, error) {
	// Decode the calendar data
	cal, err := ical.DecodeCalendar(event.RawData)
	if err != nil {
		return nil, fmt.Errorf("decoding calendar: %w", err)
	}
	
	// Remove the METHOD property
	cal.Props.Del("METHOD")
	
	// Re-encode the calendar
	rawData, err := ical.EncodeCalendar(cal)
	if err != nil {
		return nil, fmt.Errorf("encoding calendar: %w", err)
	}
	
	// Create a new event with the modified data
	preparedEvent := *event
	preparedEvent.RawData = rawData
	
	return &preparedEvent, nil
}
