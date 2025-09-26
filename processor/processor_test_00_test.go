package processor

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/storage"
)

func TestProcessorTest00NoCalendarData(t *testing.T) {
	// Create an in-memory storage
	store := storage.NewMemoryStorage()
	processor := NewProcessor(store, true)

	// Read email with no calendar data
	emailFile, err := os.Open("../test/maildir/cur/test-0.eml")
	if err != nil {
		t.Fatalf("Error opening test-0.eml: %v", err)
	}
	defer emailFile.Close()

	// Process the email
	msg, err := processor.ProcessEmail(emailFile)
	if err != nil {
		t.Fatalf("Error processing email: %v", err)
	}
	t.Logf("Mail processing result: %s", msg)

	// Check that no events were stored since this mail has no calendar data
	count := store.GetEventCount()
	if count != 0 {
		t.Errorf("Expected 0 events to be stored, but got %d", count)
	}

	// Try to get events and verify there are none
	events, err := store.ListEvents()
	if err != nil {
		t.Fatalf("Error listing events: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("Expected no events in storage, but found %d", len(events))
	}
}
