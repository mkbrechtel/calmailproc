package processor

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/storage/memory"
)

// TestProcessorTest16InvalidDTSTAMPFormat tests handling of calendar events with incorrectly formatted DTSTAMP values
func TestProcessorTest16InvalidDTSTAMPFormat(t *testing.T) {
	// Create an in-memory storage
	store := memory.NewMemoryStorage()
	processor := NewProcessor(store, true)

	// Part 1: First email should be processed successfully
	mailFile1, err := os.Open("../test/maildir/cur/test-16-1.eml")
	if err != nil {
		t.Fatalf("Error opening test-16-1.eml: %v", err)
	}
	defer mailFile1.Close()

	// Process the first email - it should succeed
	result, err := processor.ProcessEmail(mailFile1)
	if err != nil {
		t.Fatalf("Unexpected error processing first email: %v", err)
	}

	// Verify it was stored successfully
	if result != "Stored new event with UID bahn2023-07-01125700" {
		t.Errorf("Expected success message for first email, got: %s", result)
	}

	// Part 2: Second email should fail when compared to the first event
	mailFile2, err := os.Open("../test/maildir/cur/test-16-2.eml")
	if err != nil {
		t.Fatalf("Error opening test-16-2.eml: %v", err)
	}
	defer mailFile2.Close()

	// Process the second email - we expect an error during comparison
	_, err = processor.ProcessEmail(mailFile2)
	
	// Verify that we got an error
	if err == nil {
		t.Fatal("Expected an error when processing second email with incompatible DTSTAMP format, but got none")
	}

	// Log the actual error
	t.Logf("Received error as expected: %v", err)

	// The error should be related to comparing events with invalid DTSTAMP format
	if containsAny(err.Error(), []string{"comparing events", "DTSTAMP", "parsing time"}) {
		t.Logf("Error message contains expected terms related to DTSTAMP parsing error")
	} else {
		t.Errorf("Error message does not match expected format: %v", err)
	}

	// Verify only one event was stored (the first one)
	events, err := store.ListEvents()
	if err != nil {
		t.Fatalf("Error listing events: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event to be stored, got %d", len(events))
	}

	// Check that the stored event has the expected UID
	if len(events) > 0 && events[0].UID != "bahn2023-07-01125700" {
		t.Errorf("Stored event has incorrect UID: %s", events[0].UID)
	}
}