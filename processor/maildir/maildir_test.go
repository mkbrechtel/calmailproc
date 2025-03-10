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
	os.Mkdir(filepath.Join(tempDir, "new"), 0755)
	os.Mkdir(filepath.Join(tempDir, "cur"), 0755)
	os.Mkdir(filepath.Join(tempDir, "tmp"), 0755)

	// Create a processor with memory storage
	store := memory.NewMemoryStorage()
	proc := processor.NewProcessor(store, true)

	// Process the empty maildir
	err = Process(tempDir, proc, false, false, false)
	if err != nil {
		t.Errorf("Expected no error for empty maildir, but got: %v", err)
	}
}