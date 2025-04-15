package processor

import (
	"fmt"
	"io"

	"github.com/mkbrechtel/calmailproc/manager"
	"github.com/mkbrechtel/calmailproc/parser/email"
	"github.com/mkbrechtel/calmailproc/storage"
)

// Processor handles processing emails with calendar events
type Processor struct {
	Storage         storage.Storage
	ProcessReplies  bool // Whether to process METHOD:REPLY to update attendee status
	CalendarManager manager.Calendar
}

// NewProcessor creates a new processor with the given storage
func NewProcessor(storage storage.Storage, processReplies bool) *Processor {
	return &Processor{
		Storage:         storage,
		ProcessReplies:  processReplies,
		CalendarManager: manager.NewDefaultManager(),
	}
}

// ProcessEmail parses an email from an io.Reader and outputs the result
// based on the specified format (JSON or plain text)
func (p *Processor) ProcessEmail(r io.Reader, jsonOutput bool, storeEvent bool, sourceDescription string) error {
	parsedEmail, err := email.Parse(r, sourceDescription)
	if err != nil {
		return fmt.Errorf("parsing email: %w", err)
	}

	// Process the calendar event if one was found
	if storeEvent && parsedEmail.HasCalendar && parsedEmail.Event.UID != "" {
		// Check if this is a METHOD:REPLY
		if parsedEmail.Event.Method == "REPLY" {
			if !p.ProcessReplies {
				// Skip storing REPLY events when ProcessReplies is false
				fmt.Println("Ignoring calendar REPLY method as configured")
			} else {
				// Try to find the existing event to update attendee status
				existingEvent, err := p.Storage.GetEvent(parsedEmail.Event.UID)
				if err == nil && existingEvent != nil {
					// Process the reply to update attendee status
					if err := p.CalendarManager.UpdateAttendeeStatus(parsedEmail.Event, existingEvent); err != nil {
						fmt.Printf("Warning: Failed to update attendee status: %v\n", err)
						
						// If attendee update fails, store the event normally
						if err := p.Storage.StoreEvent(parsedEmail.Event); err != nil {
							return fmt.Errorf("storing event: %w", err)
						}
					} else {
						// Store the updated event
						if err := p.Storage.StoreEvent(existingEvent); err != nil {
							return fmt.Errorf("storing updated event: %w", err)
						}
					}
				} else {
					// No existing event found, store the new one
					if err := p.Storage.StoreEvent(parsedEmail.Event); err != nil {
						return fmt.Errorf("storing event: %w", err)
					}
				}
			}
		} else {
			// For non-REPLY methods
			// Check for existing event with the same UID
			existingEvent, err := p.Storage.GetEvent(parsedEmail.Event.UID)
			if err == nil && existingEvent != nil {
				// Only update if the sequence number is higher or equal (equal for backward compatibility)
				if parsedEmail.Event.Sequence < existingEvent.Sequence {
					fmt.Printf("%s - Event with lower sequence number (%d < %d) | %s\n", 
						sourceDescription, parsedEmail.Event.Sequence, existingEvent.Sequence, 
						parsedEmail.Event.UID)
				} else {
					// Store the event with higher/equal sequence
					if err := p.Storage.StoreEvent(parsedEmail.Event); err != nil {
						return fmt.Errorf("storing event: %w", err)
					}
				}
			} else {
				// No existing event found, store the new one
				if err := p.Storage.StoreEvent(parsedEmail.Event); err != nil {
					return fmt.Errorf("storing event: %w", err)
				}
			}
		}
	}

	// Only output the regular message if this is a calendar event
	// and it wasn't already reported as having a lower sequence number
	if parsedEmail.HasCalendar && 
		!(parsedEmail.Event.Sequence < existingEventSequence(p.Storage, parsedEmail.Event.UID)) {
		if jsonOutput {
			outputJSON(parsedEmail)
		} else {
			outputPlainText(parsedEmail)
		}
	}

	return nil
}

// outputJSON prints email information in JSON format
func outputJSON(parsedEmail *email.Email) {
	fmt.Println("{")
	fmt.Printf("  \"subject\": %q,\n", parsedEmail.Subject)
	fmt.Printf("  \"from\": %q,\n", parsedEmail.From)
	fmt.Printf("  \"to\": %q,\n", parsedEmail.To)
	fmt.Printf("  \"date\": %q,\n", parsedEmail.Date.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Printf("  \"has_calendar\": %t", parsedEmail.HasCalendar)

	if parsedEmail.HasCalendar {
		fmt.Printf(",\n  \"event\": {\n")
		fmt.Printf("    \"uid\": %q,\n", parsedEmail.Event.UID)
		fmt.Printf("    \"summary\": %q,\n", parsedEmail.Event.Summary)
		fmt.Printf("    \"sequence\": %d", parsedEmail.Event.Sequence)
		if parsedEmail.Event.Method != "" {
			fmt.Printf(",\n    \"method\": %q", parsedEmail.Event.Method)
		}
		fmt.Printf("\n  }")
	}

	fmt.Println("\n}")
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

// outputPlainText prints email information in plain text format
func outputPlainText(parsedEmail *email.Email) {
	// Only output if a calendar event was found
	if parsedEmail.HasCalendar {
		// Format: source | event summary | sequence | UID
		fmt.Printf("%s | %s | %d | %s\n", 
			parsedEmail.SourceDescription,
			parsedEmail.Event.Summary,
			parsedEmail.Event.Sequence,
			parsedEmail.Event.UID)
	}
}