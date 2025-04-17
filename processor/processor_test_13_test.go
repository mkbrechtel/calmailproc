package processor

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/storage/memory"
)

func TestProcessorTest13SpecificationViolation(t *testing.T) {
	// Create an in-memory storage
	store := memory.NewMemoryStorage()
	processor := NewProcessor(store, true)

	// Read the email file containing iCalendar specification violation (multiple URL properties)
	mailFile, err := os.Open("../test/maildir/cur/test-13.eml")
	if err != nil {
		t.Fatalf("Error opening test-13.eml: %v", err)
	}
	defer mailFile.Close()

	// Process the email - we expect an error
	_, err = processor.ProcessEmail(mailFile)
	
	// Verify that we got an error
	if err == nil {
		t.Fatal("Expected an error when processing calendar data with specification violation, but got none")
	}

	// Log the actual error
	t.Logf("Received error as expected: %v", err)

	// The error should be related to validation error for multiple URL properties
	if containsAny(err.Error(), []string{"validation", "error", "URL", "property", "got 2"}) {
		t.Logf("Error message contains expected terms related to iCalendar specification violation")
	} else {
		t.Errorf("Error message does not match expected format: %v", err)
	}

	// Verify no events were stored
	events, err := store.ListEvents()
	if err != nil {
		t.Fatalf("Error listing events: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("Expected 0 events to be stored, got %d", len(events))
	}
}