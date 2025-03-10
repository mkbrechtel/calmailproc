package maildir

import (
	"os"
	"path/filepath"
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

func TestProcess_EmptyMaildir(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "maildir_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create the maildir structure (empty)
	if err := os.Mkdir(filepath.Join(tempDir, "new"), 0755); err != nil {
		t.Fatalf("Failed to create new directory: %v", err)
	}
	if err := os.Mkdir(filepath.Join(tempDir, "cur"), 0755); err != nil {
		t.Fatalf("Failed to create cur directory: %v", err)
	}
	if err := os.Mkdir(filepath.Join(tempDir, "tmp"), 0755); err != nil {
		t.Fatalf("Failed to create tmp directory: %v", err)
	}

	// Create a processor with memory storage
	store := memory.NewMemoryStorage()
	proc := processor.NewProcessor(store, true)

	// Process the empty maildir
	err = Process(tempDir, proc, false, false, false)
	if err != nil {
		t.Errorf("Expected no error for empty maildir, but got: %v", err)
	}
}