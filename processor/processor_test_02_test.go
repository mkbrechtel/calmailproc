package processor

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/parser/ical"
	"github.com/mkbrechtel/calmailproc/storage/memory"
)

func TestProcessorTest02UpdateDateTime(t *testing.T) {
	// Create an in-memory storage
	store := memory.NewMemoryStorage()
	processor := NewProcessor(store, true)

	// Read initial event email
	initialFile, err := os.Open("../test/maildir/cur/test-02-1.eml")
	if err != nil {
		t.Fatalf("Error opening test-02-1.eml: %v", err)
	}
	defer initialFile.Close()

	// Process the initial email
	msg, err := processor.ProcessEmail(initialFile)
	if err != nil {
		t.Fatalf("Error processing initial email: %v", err)
	}
	t.Logf("Initial mail processing result: %s", msg)

	// Verify one event was created
	count := store.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event after initial mail, got %d", count)
	}

	// Get the event and check its properties
	events, err := store.ListEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("Error listing events: %v", err)
	}

	event := events[0]
	if event.Summary != "Test 2" {
		t.Errorf("Expected summary 'Test 2', got '%s'", event.Summary)
	}

	// Extract initial date/time from iCalendar data
	initialCal, err := ical.DecodeCalendar(event.RawData)
	if err != nil {
		t.Fatalf("Error decoding initial event: %v", err)
	}

	var initialDtstart, initialDtend string
	for _, component := range initialCal.Children {
		if component.Name == "VEVENT" {
			dtstart := component.Props.Get("DTSTART")
			if dtstart != nil {
				initialDtstart = dtstart.Value
			}

			dtend := component.Props.Get("DTEND")
			if dtend != nil {
				initialDtend = dtend.Value
			}
			break
		}
	}

	// Read update email
	updateFile, err := os.Open("../test/maildir/cur/test-02-2.eml")
	if err != nil {
		t.Fatalf("Error opening test-02-2.eml: %v", err)
	}
	defer updateFile.Close()

	// Process the update email
	msg, err = processor.ProcessEmail(updateFile)
	if err != nil {
		t.Fatalf("Error processing update email: %v", err)
	}
	t.Logf("Update mail processing result: %s", msg)

	// Verify we still have one event (just updated, not a new one)
	count = store.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event after update mail, got %d", count)
	}

	// Get the event again and check its date/time was updated
	events, err = store.ListEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("Error listing events after update: %v", err)
	}

	updatedEvent := events[0]

	// Check updated event details
	updatedCal, err := ical.DecodeCalendar(updatedEvent.RawData)
	if err != nil {
		t.Fatalf("Error decoding updated event: %v", err)
	}

	var updatedDtstart, updatedDtend string
	for _, component := range updatedCal.Children {
		if component.Name == "VEVENT" {
			dtstart := component.Props.Get("DTSTART")
			if dtstart != nil {
				updatedDtstart = dtstart.Value
			}

			dtend := component.Props.Get("DTEND")
			if dtend != nil {
				updatedDtend = dtend.Value
			}
			break
		}
	}

	// Verify date/time has changed
	if initialDtstart == updatedDtstart {
		t.Errorf("Expected start time to change, but got same value: %s", initialDtstart)
	}

	if initialDtend == updatedDtend {
		t.Errorf("Expected end time to change, but got same value: %s", initialDtend)
	}

	// Check sequence number increased
	var updatedSequence string
	for _, component := range updatedCal.Children {
		if component.Name == "VEVENT" {
			seq := component.Props.Get("SEQUENCE")
			if seq != nil {
				updatedSequence = seq.Value
				break
			}
		}
	}

	if updatedSequence != "1" {
		t.Errorf("Expected sequence number 1, got %s", updatedSequence)
	}
}
