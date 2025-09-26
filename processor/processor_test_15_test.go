package processor

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/storage"
)

func TestProcessorTest15DuplicateDTSTAMP(t *testing.T) {
	// Create an in-memory storage
	store := storage.NewMemoryStorage()
	processor := NewProcessor(store, true)

	// Read the email file containing iCalendar data with duplicate DTSTAMP properties
	mailFile, err := os.Open("../test/maildir/cur/test-15.eml")
	if err != nil {
		t.Fatalf("Error opening test-15.eml: %v", err)
	}
	defer mailFile.Close()

	// Process the email - we expect an error
	_, err = processor.ProcessEmail(mailFile)
	
	// Verify that we got an error
	if err == nil {
		t.Fatal("Expected an error when processing calendar data with duplicate DTSTAMP properties, but got none")
	}

	// Log the actual error
	t.Logf("Received error as expected: %v", err)

	// The error should be related to validation error for duplicate DTSTAMP properties
	if containsAny(err.Error(), []string{"validation", "error", "DTSTAMP", "property", "got 2"}) {
		t.Logf("Error message contains expected terms related to duplicate DTSTAMP properties")
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