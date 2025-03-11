package maildir

import (
	"os"
	"strings"
	"testing"

	"github.com/mkbrechtel/calmailproc/processor"
	"github.com/mkbrechtel/calmailproc/storage/memory"
)

func TestProcess_InvalidPath(t *testing.T) {
	// Create a processor with memory storage
	store := memory.NewMemoryStorage()
	proc := processor.NewProcessor(store, true)

	// Test with a non-existent path
	err := Process("/path/that/does/not/exist", proc, false, false, false)
	if err == nil {
		t.Errorf("Expected error for non-existent path, but got nil")
	}
}

func TestProcess_ExistingMaildir(t *testing.T) {
	// Use the existing test maildir directory
	testMaildir := "../../test/maildir"
	
	// Ensure the test maildir exists
	if _, err := os.Stat(testMaildir); os.IsNotExist(err) {
		t.Fatalf("Test maildir not found at %s", testMaildir)
	}

	// Create a processor with memory storage
	store := memory.NewMemoryStorage()
	proc := processor.NewProcessor(store, true)

	// Process the test maildir
	err := Process(testMaildir, proc, false, true, false)
	if err != nil {
		t.Errorf("Expected no error processing test maildir, but got: %v", err)
	}
	
	// Verify that we processed files
	events, err := store.ListEvents()
	if err != nil {
		t.Errorf("Error listing events: %v", err)
	}
	if len(events) == 0 {
		t.Errorf("Expected to process and store calendar events, but none were found")
	}
}

// TestProcess_MalformedEmails specifically tests handling of malformed emails
func TestProcess_MalformedEmails(t *testing.T) {
	// Use the existing test maildir directory
	testMaildir := "../../test/maildir"
	
	// Create a processor with memory storage
	store := memory.NewMemoryStorage()
	proc := processor.NewProcessor(store, true)
	
	// Process the test maildir with the store flag to ensure we attempt to store calendar events
	err := Process(testMaildir, proc, false, true, true) // Verbose mode to see outputs
	if err != nil {
		t.Errorf("Expected no error processing maildir with malformed emails, but got: %v", err)
	}
	
	// Look for recovered UIDs in the events
	var foundRecoveredEvent bool
	events, err := store.ListEvents()
	if err != nil {
		t.Errorf("Error listing events: %v", err)
	}
	for _, event := range events {
		if event.UID == "" {
			t.Errorf("Found event with empty UID, which shouldn't happen with recovery logic")
		}
		
		// Check if this is a recovered event by looking at the UID pattern
		if len(event.UID) >= 12 && strings.HasPrefix(event.UID, "recovered-uid") {
			foundRecoveredEvent = true
		}
	}
	
	// Expect to find at least one recovered event (from example-mail-15.eml)
	if !foundRecoveredEvent {
		t.Errorf("Expected to find recovered calendar event, but none found")
	}
}