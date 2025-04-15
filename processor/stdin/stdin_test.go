package stdin

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mkbrechtel/calmailproc/processor"
	"github.com/mkbrechtel/calmailproc/storage/memory"
)

func TestProcessReader(t *testing.T) {
	// Create a test memory storage
	store := memory.NewMemoryStorage()
	
	// Create processor
	proc := processor.NewProcessor(store, true)
	
	// Get test email file
	testDataPath := filepath.Join("..", "..", "test", "maildir", "cur", "example-mail-01.eml")
	emailBytes, err := os.ReadFile(testDataPath)
	if err != nil {
		t.Fatalf("Failed to read test email file: %v", err)
	}
	
	// Create reader from test file
	emailReader := bytes.NewReader(emailBytes)
	
	// Process the email
	err = ProcessReader(emailReader, proc)
	if err != nil {
		t.Fatalf("ProcessReader failed: %v", err)
	}
	
	// Verify event was stored (basic smoke test)
	events, err := store.ListEvents()
	if err != nil {
		t.Fatalf("Failed to list events: %v", err)
	}
	
	if len(events) != 1 {
		t.Errorf("Expected 1 event to be stored, got %d", len(events))
	}
}