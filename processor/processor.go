package processor

import (
	"fmt"
	"io"

	"github.com/mkbrechtel/calmailproc/parser"
	"github.com/mkbrechtel/calmailproc/storage"
)

// Processor handles processing emails with calendar events
type Processor struct {
	Storage        storage.Storage
	ProcessReplies bool // Whether to process METHOD:REPLY to update attendee status
}

// NewProcessor creates a new processor with the given storage
func NewProcessor(storage storage.Storage, processReplies bool) *Processor {
	return &Processor{
		Storage:        storage,
		ProcessReplies: processReplies,
	}
}

// ProcessEmail parses an email from an io.Reader and outputs the result
// based on the specified format (JSON or plain text)
func (p *Processor) ProcessEmail(r io.Reader, jsonOutput bool, storeEvent bool) error {
	email, err := parser.ParseEmail(r)
	if err != nil {
		return fmt.Errorf("parsing email: %w", err)
	}

	// Process the calendar event if one was found
	if storeEvent && email.HasCalendar && email.Event.UID != "" {
		// Check if this is a METHOD:REPLY that we should ignore
		if email.Event.Method == "REPLY" && !p.ProcessReplies {
			// Skip storing REPLY events when ProcessReplies is false
			fmt.Println("Ignoring calendar REPLY method as configured")
		} else {
			// Check for existing event with the same UID
			existingEvent, err := p.Storage.GetEvent(email.Event.UID)
			if err == nil && existingEvent != nil {
				// Only update if the sequence number is higher or equal (equal for backward compatibility)
				if email.Event.Sequence < existingEvent.Sequence {
					fmt.Printf("Ignoring event update with lower sequence number (%d < %d)\n", 
						email.Event.Sequence, existingEvent.Sequence)
				} else {
					// Store the event with higher/equal sequence
					if err := p.Storage.StoreEvent(&email.Event); err != nil {
						return fmt.Errorf("storing event: %w", err)
					}
				}
			} else {
				// No existing event found, store the new one
				if err := p.Storage.StoreEvent(&email.Event); err != nil {
					return fmt.Errorf("storing event: %w", err)
				}
			}
		}
	}

	if jsonOutput {
		outputJSON(email)
	} else {
		outputPlainText(email)
	}

	return nil
}

// outputJSON prints email information in JSON format
func outputJSON(email *parser.Email) {
	fmt.Println("{")
	fmt.Printf("  \"subject\": %q,\n", email.Subject)
	fmt.Printf("  \"from\": %q,\n", email.From)
	fmt.Printf("  \"to\": %q,\n", email.To)
	fmt.Printf("  \"date\": %q,\n", email.Date.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Printf("  \"has_calendar\": %t", email.HasCalendar)

	if email.HasCalendar {
		fmt.Printf(",\n  \"event\": {\n")
		fmt.Printf("    \"uid\": %q,\n", email.Event.UID)
		fmt.Printf("    \"summary\": %q,\n", email.Event.Summary)
		fmt.Printf("    \"sequence\": %d", email.Event.Sequence)
		if email.Event.Method != "" {
			fmt.Printf(",\n    \"method\": %q", email.Event.Method)
		}
		fmt.Printf("\n  }")
	}

	fmt.Println("\n}")
}

// outputPlainText prints email information in plain text format
func outputPlainText(email *parser.Email) {
	fmt.Printf("Subject: %s\n", email.Subject)
	fmt.Printf("From: %s\n", email.From)
	fmt.Printf("To: %s\n", email.To)
	fmt.Printf("Date: %s\n", email.Date.Format("2006-01-02 15:04:05"))

	// Print calendar information if available
	if email.HasCalendar {
		fmt.Println("\nCalendar Event:")
		fmt.Printf("  UID: %s\n", email.Event.UID)
		fmt.Printf("  Summary: %s\n", email.Event.Summary)
		fmt.Printf("  Sequence: %d\n", email.Event.Sequence)
		if email.Event.Method != "" {
			fmt.Printf("  Method: %s\n", email.Event.Method)
		}
	}
}
