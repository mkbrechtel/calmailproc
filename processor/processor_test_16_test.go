package processor

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/storage/memory"
)

// TestProcessorTest16InvalidDTSTAMPFormat tests handling of calendar events with non-standard formatted DTSTAMP values
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

	// Part 2: Second email should also be processed successfully
	mailFile2, err := os.Open("../test/maildir/cur/test-16-2.eml")
	if err != nil {
		t.Fatalf("Error opening test-16-2.eml: %v", err)
	}
	defer mailFile2.Close()

	// Process the second email - it should succeed now with ISO format support
	result, err = processor.ProcessEmail(mailFile2)
	
	// Verify that we did not get an error
	if err != nil {
		t.Fatalf("Unexpected error processing second email: %v", err)
	}

	// Check for an update message (our implementation treats it as an update)
	expectedResult := "Updated event with UID bahn2023-07-01125700, new sequence: 0"
	if result != expectedResult {
		t.Errorf("Expected '%s' for second email, got: %s", expectedResult, result)
	}

	// Verify only one event was stored (both emails reference the same event)
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