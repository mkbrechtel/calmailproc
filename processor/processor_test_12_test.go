package processor

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/storage/memory"
)

func TestProcessorTest12InvalidCalendarData(t *testing.T) {
	// Create an in-memory storage
	store := memory.NewMemoryStorage()
	processor := NewProcessor(store, true)

	// Read the email file containing invalid calendar data
	mailFile, err := os.Open("../test/maildir/cur/test-12.eml")
	if err != nil {
		t.Fatalf("Error opening test-12.eml: %v", err)
	}
	defer mailFile.Close()

	// Process the email - we expect an error
	_, err = processor.ProcessEmail(mailFile)
	
	// Verify that we got an error
	if err == nil {
		t.Fatal("Expected an error when processing invalid calendar data, but got none")
	}

	// Log the actual error
	t.Logf("Received error as expected: %v", err)

	// The error should be related to parsing iCal data
	if containsAny(err.Error(), []string{"panic", "iCal", "calendar", "index out of range"}) {
		t.Logf("Error message contains expected terms related to parsing invalid iCal data")
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

// Helper function to check if a string contains any of the given substrings
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}