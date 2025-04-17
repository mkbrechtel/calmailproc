package processor

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/parser/ical"
	"github.com/mkbrechtel/calmailproc/storage/memory"
)

func TestProcessorTest10InlineCalendarContent(t *testing.T) {
	// Create an in-memory storage
	store := memory.NewMemoryStorage()
	processor := NewProcessor(store, true)

	// Read the email file containing inline calendar content
	mailFile, err := os.Open("../test/maildir/cur/test-10.eml")
	if err != nil {
		t.Fatalf("Error opening test-10.eml: %v", err)
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
	if event.Summary != "Test 10" {
		t.Errorf("Expected summary 'Test 10', got '%s'", event.Summary)
	}

	// Parse the calendar to check more detailed properties
	cal, err := ical.DecodeCalendar(event.RawData)
	if err != nil {
		t.Fatalf("Error decoding calendar data: %v", err)
	}

	// Verify TENTATIVE status and other properties
	var hasTentativeStatus bool
	var hasCorrectOrganizer bool
	var hasCorrectAttendee bool
	var hasAlarm bool

	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		// Check X-MICROSOFT-CDO-BUSYSTATUS for TENTATIVE
		busyStatus := component.Props.Get("X-MICROSOFT-CDO-BUSYSTATUS")
		if busyStatus != nil && busyStatus.Value == "TENTATIVE" {
			hasTentativeStatus = true
		}

		// Check organizer
		organizer := component.Props.Get("ORGANIZER")
		if organizer != nil && organizer.Value == "mailto:example@example.com" {
			hasCorrectOrganizer = true
		}

		// Check attendee
		for _, attendee := range component.Props.Values("ATTENDEE") {
			if attendee.Value == "mailto:markus.brechtel@uk-koeln.de" {
				hasCorrectAttendee = true
				break
			}
		}

		// Check for VALARM component
		for _, child := range component.Children {
			if child.Name == "VALARM" {
				hasAlarm = true
				break
			}
		}
	}

	if !hasTentativeStatus {
		t.Error("Event does not have TENTATIVE status based on X-MICROSOFT-CDO-BUSYSTATUS")
	}

	if !hasCorrectOrganizer {
		t.Error("Event does not have the correct organizer")
	}

	if !hasCorrectAttendee {
		t.Error("Event does not have the correct attendee")
	}

	if !hasAlarm {
		t.Error("Event does not have the expected VALARM component")
	}
}