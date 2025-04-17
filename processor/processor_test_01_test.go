package processor

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/parser/ical"
	"github.com/mkbrechtel/calmailproc/storage/memory"
)

func TestProcessorTest01CreateThenCancel(t *testing.T) {
	// Create an in-memory storage
	store := memory.NewMemoryStorage()
	processor := NewProcessor(store, true)

	// Read creation email
	createFile, err := os.Open("../test/maildir/cur/test-01-1.eml")
	if err != nil {
		t.Fatalf("Error opening test-01-1.eml: %v", err)
	}
	defer createFile.Close()

	// Process the creation email
	msg, err := processor.ProcessEmail(createFile)
	if err != nil {
		t.Fatalf("Error processing creation email: %v", err)
	}
	t.Logf("Creation mail processing result: %s", msg)

	// Verify one event was created
	count := store.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event after creation mail, got %d", count)
	}

	// Get the event and check its properties
	events, err := store.ListEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("Error listing events: %v", err)
	}

	event := events[0]
	if event.Summary != "Test Event 1" {
		t.Errorf("Expected summary 'Test Event 1', got '%s'", event.Summary)
	}

	// Check event status is CONFIRMED
	cal, err := ical.DecodeCalendar(event.RawData)
	if err != nil {
		t.Fatalf("Error decoding event: %v", err)
	}

	var initialStatus string
	for _, component := range cal.Children {
		if component.Name == "VEVENT" {
			status := component.Props.Get("STATUS")
			if status != nil {
				initialStatus = status.Value
				break
			}
		}
	}

	if initialStatus != "CONFIRMED" {
		t.Errorf("Expected status CONFIRMED, got %s", initialStatus)
	}

	// Read cancellation email
	cancelFile, err := os.Open("../test/maildir/cur/test-01-2.eml")
	if err != nil {
		t.Fatalf("Error opening test-01-2.eml: %v", err)
	}
	defer cancelFile.Close()

	// Process the cancellation email
	msg, err = processor.ProcessEmail(cancelFile)
	if err != nil {
		t.Fatalf("Error processing cancellation email: %v", err)
	}
	t.Logf("Cancellation mail processing result: %s", msg)

	// Verify we still have one event (just updated, not a new one)
	count = store.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event after cancellation mail, got %d", count)
	}

	// Get the event again and check its status was updated to CANCELLED
	events, err = store.ListEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("Error listing events after cancellation: %v", err)
	}

	event = events[0]
	// Summary may include "Cancelled:" prefix or "Abgesagt:" (German for "Cancelled:") depending on implementation
	if event.Summary != "Test Event 1" && 
	   event.Summary != "Cancelled: Test Event 1" && 
	   event.Summary != "Abgesagt: Test Event 1" {
		t.Errorf("Unexpected summary after cancellation: '%s'", event.Summary)
	}

	// Check event status is now CANCELLED
	cal, err = ical.DecodeCalendar(event.RawData)
	if err != nil {
		t.Fatalf("Error decoding event after cancellation: %v", err)
	}

	var finalStatus string
	for _, component := range cal.Children {
		if component.Name == "VEVENT" {
			status := component.Props.Get("STATUS")
			if status != nil {
				finalStatus = status.Value
				break
			}
		}
	}

	if finalStatus != "CANCELLED" {
		t.Errorf("Expected status CANCELLED, got %s", finalStatus)
	}
}
