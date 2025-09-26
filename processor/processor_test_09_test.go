package processor

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/parser/ical"
	"github.com/mkbrechtel/calmailproc/storage"
)

func TestProcessorTest09ForwardedCalendarInvitation(t *testing.T) {
	// Create an in-memory storage
	store := storage.NewMemoryStorage()
	processor := NewProcessor(store, true)

	// Read the email file containing a forwarded calendar invitation
	mailFile, err := os.Open("../test/maildir/cur/test-09.eml")
	if err != nil {
		t.Fatalf("Error opening test-09.eml: %v", err)
	}
	defer mailFile.Close()

	// Process the email
	msg, err := processor.ProcessEmail(mailFile)
	if err != nil {
		t.Fatalf("Error processing email: %v", err)
	}
	t.Logf("Mail processing result: %s", msg)

	// Check if the event was stored
	events, err := store.ListEvents()
	if err != nil {
		t.Fatalf("Error listing events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	// Get the event and check its properties
	event := events[0]

	// Check basic properties
	if event.Summary != "Test 9" {
		t.Errorf("Expected summary 'Test 9', got '%s'", event.Summary)
	}

	// Parse the calendar to check more detailed properties
	cal, err := ical.DecodeCalendar(event.RawData)
	if err != nil {
		t.Fatalf("Error decoding calendar data: %v", err)
	}

	// Verify the status is CONFIRMED
	var statusValue string
	var hasCorrectOrganizer bool
	var hasCorrectAttendee bool

	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		// Check status
		status := component.Props.Get("STATUS")
		if status != nil {
			statusValue = status.Value
		}

		// Check organizer
		organizer := component.Props.Get("ORGANIZER")
		if organizer != nil && organizer.Value == "mailto:C.Lee@med.uni-frankfurt.de" {
			hasCorrectOrganizer = true
		}

		// Check attendee
		for _, attendee := range component.Props.Values("ATTENDEE") {
			if attendee.Value == "mailto:ludwig.montag@med.uni-frankfurt.de" {
				hasCorrectAttendee = true
				break
			}
		}
	}

	if statusValue != "CONFIRMED" {
		t.Errorf("Expected status 'CONFIRMED', got '%s'", statusValue)
	}

	if !hasCorrectOrganizer {
		t.Error("Event does not have the correct organizer")
	}

	if !hasCorrectAttendee {
		t.Error("Event does not have the correct attendee")
	}
}