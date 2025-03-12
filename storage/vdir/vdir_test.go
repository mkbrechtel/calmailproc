package vdir

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mkbrechtel/calmailproc/parser/ical"
)

func TestVDirStoreAndRetrieve(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "vdir-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a new VDirStorage
	storage, err := NewVDirStorage(tempDir)
	if err != nil {
		t.Fatalf("Failed to create VDirStorage: %v", err)
	}

	// Create a test event
	testUID := "test-event-123@example.com"
	testEvent := &ical.Event{
		UID:     testUID,
		Summary: "Test Event",
		RawData: []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//example//test//EN\r\nBEGIN:VEVENT\r\nUID:test-event-123@example.com\r\nSUMMARY:Test Event\r\nDTSTART:20240101T100000Z\r\nDTEND:20240101T110000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"),
	}

	// Store the event
	err = storage.StoreEvent(testEvent)
	if err != nil {
		t.Fatalf("Failed to store event: %v", err)
	}

	// Verify the file was created with a hashed name
	expectedHash := HashFilename(testUID)
	expectedFilename := expectedHash + ".ics"
	expectedPath := filepath.Join(tempDir, expectedFilename)
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file %s not found", expectedPath)
	}

	// Retrieve the event
	retrievedEvent, err := storage.GetEvent(testUID)
	if err != nil {
		t.Fatalf("Failed to retrieve event: %v", err)
	}

	// Verify event data
	if retrievedEvent.UID != testUID {
		t.Errorf("Retrieved event UID mismatch: got %s, want %s", retrievedEvent.UID, testUID)
	}
	if retrievedEvent.Summary != testEvent.Summary {
		t.Errorf("Retrieved event Summary mismatch: got %s, want %s", retrievedEvent.Summary, testEvent.Summary)
	}

	// Test ListEvents
	events, err := storage.ListEvents()
	if err != nil {
		t.Fatalf("Failed to list events: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	// Test DeleteEvent
	err = storage.DeleteEvent(testUID)
	if err != nil {
		t.Fatalf("Failed to delete event: %v", err)
	}

	// Verify the file was deleted
	if _, err := os.Stat(expectedPath); !os.IsNotExist(err) {
		t.Errorf("Expected file %s to be deleted, but it still exists", expectedPath)
	}
}

func TestVDirMultipleEvents(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "vdir-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a new VDirStorage
	storage, err := NewVDirStorage(tempDir)
	if err != nil {
		t.Fatalf("Failed to create VDirStorage: %v", err)
	}

	// Create multiple test events
	events := []*ical.Event{
		{
			UID:     "event1@example.com",
			Summary: "Event 1",
			RawData: []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//example//test//EN\r\nBEGIN:VEVENT\r\nUID:event1@example.com\r\nSUMMARY:Event 1\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"),
		},
		{
			UID:     "event2@example.com",
			Summary: "Event 2",
			RawData: []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//example//test//EN\r\nBEGIN:VEVENT\r\nUID:event2@example.com\r\nSUMMARY:Event 2\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"),
		},
	}

	// Store all events
	for _, event := range events {
		if err := storage.StoreEvent(event); err != nil {
			t.Fatalf("Failed to store event %s: %v", event.UID, err)
		}
	}

	// List all events
	retrievedEvents, err := storage.ListEvents()
	if err != nil {
		t.Fatalf("Failed to list events: %v", err)
	}

	// Verify count
	if len(retrievedEvents) != len(events) {
		t.Errorf("Expected %d events, got %d", len(events), len(retrievedEvents))
	}

	// Create a map of UIDs to verify each event was retrieved
	foundUIDs := make(map[string]bool)
	for _, event := range retrievedEvents {
		foundUIDs[event.UID] = true
	}

	// Verify all expected UIDs are present
	for _, event := range events {
		if !foundUIDs[event.UID] {
			t.Errorf("Event with UID %s was not retrieved", event.UID)
		}
	}
}