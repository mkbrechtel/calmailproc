package memory

import (
	"bytes"
	"testing"

	"github.com/mkbrechtel/calmailproc/parser/ical"
)

func TestMemoryStorage_Basic(t *testing.T) {
	storage := NewMemoryStorage()

	// Create a test event
	event := &ical.Event{
		UID:     "test-event-1",
		Summary: "Test Event",
		RawData: []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Example//Calendar//EN\r\nBEGIN:VEVENT\r\nUID:test-event-1\r\nSUMMARY:Test Event\r\nEND:VEVENT\r\nEND:VCALENDAR"),
	}

	// Store the event
	if err := storage.StoreEvent(event); err != nil {
		t.Fatalf("StoreEvent failed: %v", err)
	}

	// Check that it was stored
	if count := storage.GetEventCount(); count != 1 {
		t.Errorf("Expected 1 event, got %d", count)
	}

	// Retrieve the event
	retrievedEvent, err := storage.GetEvent(event.UID)
	if err != nil {
		t.Fatalf("GetEvent failed: %v", err)
	}

	// Check that the retrieved event matches
	if retrievedEvent.UID != event.UID {
		t.Errorf("Retrieved event UID %s does not match original %s", retrievedEvent.UID, event.UID)
	}
	if retrievedEvent.Summary != event.Summary {
		t.Errorf("Retrieved event Summary %s does not match original %s", retrievedEvent.Summary, event.Summary)
	}
	if !bytes.Equal(retrievedEvent.RawData, event.RawData) {
		t.Errorf("Retrieved event RawData does not match original")
	}

	// List events
	events, err := storage.ListEvents()
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("Expected 1 event in list, got %d", len(events))
	}

	// Delete the event
	if err := storage.DeleteEvent(event.UID); err != nil {
		t.Fatalf("DeleteEvent failed: %v", err)
	}

	// Check that it was deleted
	if count := storage.GetEventCount(); count != 0 {
		t.Errorf("Expected 0 events after deletion, got %d", count)
	}

	// Try to retrieve the deleted event
	_, err = storage.GetEvent(event.UID)
	if err == nil {
		t.Errorf("GetEvent should fail for deleted event")
	}
}

func TestMemoryStorage_RecurringEventUpdate(t *testing.T) {
	// With our simplified memory implementation, we don't need to test
	// the complex recurring event update logic here since that's handled
	// by the processor, not the storage.
	
	storage := NewMemoryStorage()

	// Create a master recurring event
	masterEventData := []byte(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Example//Calendar//EN
BEGIN:VEVENT
UID:recurring-event-1
SUMMARY:Recurring Meeting
DTSTART:20250301T090000Z
DTEND:20250301T100000Z
DTSTAMP:20250301T080000Z
RRULE:FREQ=WEEKLY;COUNT=4
END:VEVENT
END:VCALENDAR`)

	masterEvent := &ical.Event{
		UID:     "recurring-event-1",
		Summary: "Recurring Meeting",
		RawData: masterEventData,
	}

	// Store the master event
	if err := storage.StoreEvent(masterEvent); err != nil {
		t.Fatalf("Failed to store master event: %v", err)
	}

	// Create an update to a specific instance (second occurrence)
	updateEventData := []byte(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Example//Calendar//EN
METHOD:REQUEST
BEGIN:VEVENT
UID:recurring-event-1
SUMMARY:Updated Recurring Meeting
DTSTART:20250308T100000Z
DTEND:20250308T110000Z
DTSTAMP:20250308T080000Z
RECURRENCE-ID:20250308T090000Z
END:VEVENT
END:VCALENDAR`)

	updateEvent := &ical.Event{
		UID:     "recurring-event-1",
		Summary: "Updated Recurring Meeting",
		Method:  "REQUEST",
		RawData: updateEventData,
	}

	// Store the update - with simple memory implementation, this will just replace the event
	if err := storage.StoreEvent(updateEvent); err != nil {
		t.Fatalf("Failed to store update event: %v", err)
	}

	// Retrieve the event - should now be the updated event
	retrievedEvent, err := storage.GetEvent(masterEvent.UID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated event: %v", err)
	}

	// Check that we got the update, not the original
	if !bytes.Equal(retrievedEvent.RawData, updateEvent.RawData) {
		t.Errorf("Retrieved event should be the updated event")
	}
}

func TestMemoryStorage_CancelledEvent(t *testing.T) {
	storage := NewMemoryStorage()

	// Create a master recurring event
	masterEventData := []byte(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Example//Calendar//EN
BEGIN:VEVENT
UID:recurring-event-2
SUMMARY:Recurring Meeting
DTSTART:20250401T090000Z
DTEND:20250401T100000Z
DTSTAMP:20250401T080000Z
RRULE:FREQ=WEEKLY;COUNT=4
END:VEVENT
END:VCALENDAR`)

	masterEvent := &ical.Event{
		UID:     "recurring-event-2",
		Summary: "Recurring Meeting",
		RawData: masterEventData,
	}

	// Store the master event
	if err := storage.StoreEvent(masterEvent); err != nil {
		t.Fatalf("Failed to store master event: %v", err)
	}

	// Create a cancellation of a specific instance
	cancelEventData := []byte(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Example//Calendar//EN
METHOD:CANCEL
BEGIN:VEVENT
UID:recurring-event-2
SUMMARY:Recurring Meeting
DTSTART:20250408T090000Z
DTEND:20250408T100000Z
DTSTAMP:20250408T080000Z
RECURRENCE-ID:20250408T090000Z
STATUS:CANCELLED
END:VEVENT
END:VCALENDAR`)

	cancelEvent := &ical.Event{
		UID:     "recurring-event-2",
		Summary: "Recurring Meeting",
		Method:  "CANCEL",
		RawData: cancelEventData,
	}

	// Store the cancellation - with simple memory implementation, this will just replace the event
	if err := storage.StoreEvent(cancelEvent); err != nil {
		t.Fatalf("Failed to store cancellation event: %v", err)
	}

	// Retrieve the event - should now be the cancellation event
	retrievedEvent, err := storage.GetEvent(masterEvent.UID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated event: %v", err)
	}

	// Check that we got the cancellation, not the original
	if !bytes.Equal(retrievedEvent.RawData, cancelEvent.RawData) {
		t.Errorf("Retrieved event should be the cancellation event")
	}
}