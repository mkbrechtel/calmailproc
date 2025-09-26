package processor

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/parser/ical"
	"github.com/mkbrechtel/calmailproc/storage"
)

func TestProcessorTest04DuplicateEvents(t *testing.T) {
	// Create an in-memory storage
	store := storage.NewMemoryStorage()
	processor := NewProcessor(store, true)

	// Read first invitation mail
	firstMailFile, err := os.Open("../test/maildir/cur/test-04-1.eml")
	if err != nil {
		t.Fatalf("Error opening test-04-1.eml: %v", err)
	}
	defer firstMailFile.Close()

	// Process the first mail
	msg, err := processor.ProcessEmail(firstMailFile)
	if err != nil {
		t.Fatalf("Error processing first email: %v", err)
	}
	t.Logf("First mail processing result: %s", msg)

	// Verify one event was created
	count := store.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event after first mail, got %d", count)
	}

	// Get the event and store its properties for comparison
	events, err := store.ListEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("Error listing events: %v", err)
	}

	firstEvent := events[0]
	if firstEvent.Summary != "Test 4" {
		t.Errorf("Expected summary 'Test 4', got '%s'", firstEvent.Summary)
	}

	// Capture the UID for later verification
	eventUID := firstEvent.UID

	// Read second duplicate invitation mail
	secondMailFile, err := os.Open("../test/maildir/cur/test-04-2.eml")
	if err != nil {
		t.Fatalf("Error opening test-04-2.eml: %v", err)
	}
	defer secondMailFile.Close()

	// Process the second mail
	msg, err = processor.ProcessEmail(secondMailFile)
	if err != nil {
		t.Fatalf("Error processing second email: %v", err)
	}
	t.Logf("Second mail processing result: %s", msg)

	// Verify we still have only one event (duplicate should be recognized)
	count = store.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event after duplicate mail, got %d", count)
	}

	// Get the event again and verify no key properties changed
	events, err = store.ListEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("Error listing events after duplicate: %v", err)
	}

	secondEvent := events[0]

	// Verify the event is the same (same UID)
	if secondEvent.UID != eventUID {
		t.Errorf("Expected same UID: %s, got %s", eventUID, secondEvent.UID)
	}

	// Verify summary didn't change
	if secondEvent.Summary != "Test 4" {
		t.Errorf("Expected summary 'Test 4', got '%s'", secondEvent.Summary)
	}

	// Let's check that attendees were properly processed in both mails
	finalCal, err := ical.DecodeCalendar(secondEvent.RawData)
	if err != nil {
		t.Fatalf("Error decoding final calendar: %v", err)
	}

	// Check that we have both attendees in the final event
	attendeeCount := 0
	attendeeEmails := make(map[string]bool)

	for _, component := range finalCal.Children {
		if component.Name == "VEVENT" {
			for _, attendee := range component.Props.Values("ATTENDEE") {
				attendeeCount++
				// Extract email from mailto: format
				email := attendee.Value
				if len(email) > 7 && email[:7] == "mailto:" {
					email = email[7:]
				}
				attendeeEmails[email] = true
			}
			break
		}
	}

	// There should be at least 2 attendees per the test case
	if attendeeCount < 2 {
		t.Errorf("Expected at least 2 attendees, got %d", attendeeCount)
	}

	// Verify the expected attendees are in the list
	expectedEmails := []string{
		"markus.brechtel@uk-koeln.de",
		"brechtel@med.uni-frankfurt.de",
	}

	for _, email := range expectedEmails {
		if !attendeeEmails[email] {
			t.Errorf("Expected attendee %s not found in final event", email)
		}
	}
}
