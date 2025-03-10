package processor

import (
	"fmt"
	"io"

	"github.com/mkbrechtel/calmailproc/parser"
	"github.com/mkbrechtel/calmailproc/storage"
)

// Processor handles processing emails with calendar events
type Processor struct {
	Storage storage.Storage
}

// NewProcessor creates a new processor with the given storage
func NewProcessor(storage storage.Storage) *Processor {
	return &Processor{
		Storage: storage,
	}
}

// ProcessEmail parses an email from an io.Reader and outputs the result
// based on the specified format (JSON or plain text)
func (p *Processor) ProcessEmail(r io.Reader, jsonOutput bool, storeEvent bool) error {
	email, err := parser.ParseEmail(r)
	if err != nil {
		return fmt.Errorf("parsing email: %w", err)
	}

	// Store the event if requested and a calendar event was found
	if storeEvent && email.HasCalendar && email.Event.UID != "" {
		if err := p.Storage.StoreEvent(&email.Event); err != nil {
			return fmt.Errorf("storing event: %w", err)
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
		fmt.Printf("    \"summary\": %q\n", email.Event.Summary)
		fmt.Printf("  }")
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
	}
}
