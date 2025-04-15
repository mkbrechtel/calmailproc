package icalfile

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	goical "github.com/emersion/go-ical"
	"github.com/mkbrechtel/calmailproc/parser/ical"
)

func TestICalFileStorage(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "icalfile_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file path
	testFilePath := filepath.Join(tempDir, "test.ics")

	// Create the storage
	storage, err := NewICalFileStorage(testFilePath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Open the storage
	if err := storage.OpenAndLock(); err != nil {
		t.Fatalf("Failed to open storage: %v", err)
	}

	// Create test events
	event1, err := createTestEvent("event1", "Test Event 1", "2023-01-01T10:00:00Z", "2023-01-01T11:00:00Z")
	if err != nil {
		t.Fatalf("Failed to create test event 1: %v", err)
	}
	
	event2, err := createTestEvent("event2", "Test Event 2", "2023-01-02T10:00:00Z", "2023-01-02T11:00:00Z")
	if err != nil {
		t.Fatalf("Failed to create test event 2: %v", err)
	}

	// Test storing events
	if err := storage.StoreEvent(event1); err != nil {
		t.Fatalf("Failed to store event1: %v", err)
	}
	if err := storage.StoreEvent(event2); err != nil {
		t.Fatalf("Failed to store event2: %v", err)
	}

	// Test retrieving events
	retrieved, err := storage.GetEvent("event1")
	if err != nil {
		t.Fatalf("Failed to retrieve event1: %v", err)
	}
	if retrieved.UID != "event1" {
		t.Errorf("Expected UID 'event1', got '%s'", retrieved.UID)
	}

	// Test listing events
	events, err := storage.ListEvents()
	if err != nil {
		t.Fatalf("Failed to list events: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	// Close the storage (should write to file)
	if err := storage.WriteAndUnlock(); err != nil {
		t.Fatalf("Failed to close storage: %v", err)
	}

	// Verify the file was created
	if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
		t.Fatalf("File was not created")
	}

	// Open a new storage instance to verify persistence
	storage2, err := NewICalFileStorage(testFilePath)
	if err != nil {
		t.Fatalf("Failed to create second storage: %v", err)
	}

	// Open the storage (should read from file)
	if err := storage2.OpenAndLock(); err != nil {
		t.Fatalf("Failed to open second storage: %v", err)
	}

	// Verify events were loaded
	events, err = storage2.ListEvents()
	if err != nil {
		t.Fatalf("Failed to list events from second storage: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("Expected 2 events in second storage, got %d", len(events))
	}

	// Test deleting an event
	if err := storage2.DeleteEvent("event1"); err != nil {
		t.Fatalf("Failed to delete event: %v", err)
	}

	// Verify event was deleted
	events, err = storage2.ListEvents()
	if err != nil {
		t.Fatalf("Failed to list events after deletion: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("Expected 1 event after deletion, got %d", len(events))
	}

	// Close the second storage (should write to file)
	if err := storage2.WriteAndUnlock(); err != nil {
		t.Fatalf("Failed to close second storage: %v", err)
	}

	// Open a third storage instance to verify the deletion was persisted
	storage3, err := NewICalFileStorage(testFilePath)
	if err != nil {
		t.Fatalf("Failed to create third storage: %v", err)
	}

	// Open the storage (should read from file)
	if err := storage3.OpenAndLock(); err != nil {
		t.Fatalf("Failed to open third storage: %v", err)
	}

	// Verify one event was loaded
	events, err = storage3.ListEvents()
	if err != nil {
		t.Fatalf("Failed to list events from third storage: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("Expected 1 event in third storage, got %d", len(events))
	}
	if events[0].UID != "event2" {
		t.Errorf("Expected event with UID 'event2', got '%s'", events[0].UID)
	}

	// Close the third storage
	if err := storage3.WriteAndUnlock(); err != nil {
		t.Fatalf("Failed to close third storage: %v", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "icalfile_concurrent_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file path
	testFilePath := filepath.Join(tempDir, "concurrent.ics")

	// Create the storage
	storage, err := NewICalFileStorage(testFilePath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Open the storage
	if err := storage.OpenAndLock(); err != nil {
		t.Fatalf("Failed to open storage: %v", err)
	}

	// Create some test events first
	for i := 0; i < 5; i++ {
		event, err := createTestEvent(
			fmt.Sprintf("event%d", i),
			fmt.Sprintf("Test Event %d", i),
			fmt.Sprintf("2023-01-0%dT10:00:00Z", i+1),
			fmt.Sprintf("2023-01-0%dT11:00:00Z", i+1),
		)
		if err != nil {
			t.Fatalf("Failed to create test event %d: %v", i, err)
		}
		if err := storage.StoreEvent(event); err != nil {
			t.Fatalf("Failed to store initial event %d: %v", i, err)
		}
	}

	// Test concurrent access
	done := make(chan bool)
	errorChan := make(chan error, 20)

	// Launch goroutines to store and retrieve events concurrently
	for i := 0; i < 10; i++ {
		go func(i int) {
			// Store a new event
			eventID := fmt.Sprintf("concurrent%d", i)
			event, err := createTestEvent(
				eventID,
				fmt.Sprintf("Concurrent Event %d", i),
				fmt.Sprintf("2023-02-0%dT10:00:00Z", (i%9)+1),
				fmt.Sprintf("2023-02-0%dT11:00:00Z", (i%9)+1),
			)
			if err != nil {
				errorChan <- fmt.Errorf("failed to create concurrent event %d: %v", i, err)
				return
			}
			
			if err := storage.StoreEvent(event); err != nil {
				errorChan <- err
				return
			}

			// Retrieve existing events
			_, err = storage.ListEvents()
			if err != nil {
				errorChan <- err
				return
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case err := <-errorChan:
			t.Errorf("Concurrent operation failed: %v", err)
		case <-done:
			// Success
		}
	}

	// Close the storage
	if err := storage.WriteAndUnlock(); err != nil {
		t.Fatalf("Failed to close storage: %v", err)
	}
}

// createTestEvent creates a test event with the given parameters
func createTestEvent(uid, summary, start, end string) (*ical.Event, error) {
	// Create a minimal calendar with a VEVENT
	cal := ical.NewCalendar()
	
	// Create VEVENT component
	event := &goical.Component{
		Name: "VEVENT",
		Props: make(goical.Props),
	}
	
	// Add properties to the event
	uidProp := &goical.Prop{Name: "UID", Value: uid}
	summaryProp := &goical.Prop{Name: "SUMMARY", Value: summary}
	startProp := &goical.Prop{Name: "DTSTART", Value: start}
	endProp := &goical.Prop{Name: "DTEND", Value: end}
	
	// Add required DTSTAMP property (required by ical spec)
	stampProp := &goical.Prop{Name: "DTSTAMP", Value: time.Now().UTC().Format("20060102T150405Z")}
	
	event.Props.Set(uidProp)
	event.Props.Set(summaryProp)
	event.Props.Set(startProp)
	event.Props.Set(endProp)
	event.Props.Set(stampProp)
	
	cal.Children = append(cal.Children, event)
	
	// Encode the calendar to get raw data
	rawData, err := ical.EncodeCalendar(cal)
	if err != nil {
		return nil, fmt.Errorf("encoding calendar: %w", err)
	}
	
	// Create the event object
	result, err := ical.ParseICalData(rawData)
	if err != nil {
		return nil, fmt.Errorf("parsing calendar data: %w", err)
	}
	
	return result, nil
}