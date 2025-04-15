package maildir

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/processor"
	"github.com/mkbrechtel/calmailproc/storage/memory"
)

func TestProcess_InvalidPath(t *testing.T) {
	// Create a processor with memory storage
	store := memory.NewMemoryStorage()
	proc := processor.NewProcessor(store, true)

	// Test with a non-existent path
	err := Process("/path/that/does/not/exist", proc, false)
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
	err := Process(testMaildir, proc, false)
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
	
	// Process the test maildir with verbose mode to see outputs
	err := Process(testMaildir, proc, true) 
	if err != nil {
		t.Errorf("Expected no error processing maildir with malformed emails, but got: %v", err)
	}
	
	// We no longer expect to have recovered events with generated UIDs
	// Instead, example-mail-15.eml should be correctly identified as unparseable
	// and should not appear in the stored events at all
	
	// Count events to ensure we're not storing invalid calendar data
	events, err := store.ListEvents()
	if err != nil {
		t.Errorf("Error listing events: %v", err)
	}
	
	// Verify that all stored events have valid non-empty UIDs
	for _, event := range events {
		if event.UID == "" {
			t.Errorf("Found event with empty UID in storage, which should never happen")
		}
	}
}