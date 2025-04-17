package processor

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/parser/ical"
	"github.com/mkbrechtel/calmailproc/storage/memory"
)

func TestProcessorTest03RecurringEvents(t *testing.T) {
	// Create an in-memory storage and processor
	store := memory.NewMemoryStorage()
	processor := NewProcessor(store, true)

	// 1. Process the original recurring event mail
	recurringFile, err := os.Open("../test/maildir/cur/test-03-1.eml")
	if err != nil {
		t.Fatalf("Error opening test-03-1.eml: %v", err)
	}
	defer recurringFile.Close()

	msg, err := processor.ProcessEmail(recurringFile)
	if err != nil {
		t.Fatalf("Error processing recurring event email: %v", err)
	}
	t.Logf("Recurring event mail processing result: %s", msg)

	// Verify one event was stored
	count := store.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event after recurring mail, got %d", count)
	}

	// Get the event and check its properties
	events, err := store.ListEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("Error listing events: %v", err)
	}

	recurringEvent := events[0]
	// The recurring event UID from the test files
	recurringUID := "040000008200E00074C5B7101A82E0080000000044440AFCBB91DB0100000000000000001000000087598F58784D4541BAA76F1829CFE9A1"
	if recurringEvent.UID != recurringUID {
		t.Errorf("Wrong event UID: %s, expected %s", recurringEvent.UID, recurringUID)
	}

	// Verify event has RRULE (recurring rule)
	recurringCal, err := ical.DecodeCalendar(recurringEvent.RawData)
	if err != nil {
		t.Fatalf("Error decoding recurring event: %v", err)
	}

	hasRrule := false
	for _, component := range recurringCal.Children {
		if component.Name == "VEVENT" {
			rrule := component.Props.Get("RRULE")
			if rrule != nil {
				hasRrule = true
				break
			}
		}
	}

	if !hasRrule {
		t.Errorf("Expected recurring event to have RRULE property, but it doesn't")
	}

	// 2. Process the cancellation of a specific instance
	cancelInstanceFile, err := os.Open("../test/maildir/cur/test-03-2.eml")
	if err != nil {
		t.Fatalf("Error opening test-03-2.eml: %v", err)
	}
	defer cancelInstanceFile.Close()

	msg, err = processor.ProcessEmail(cancelInstanceFile)
	if err != nil {
		t.Fatalf("Error processing cancelled instance email: %v", err)
	}
	t.Logf("Cancelled instance mail processing result: %s", msg)

	// Verify we still have 1 event (same UID, just updated)
	count = store.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event after cancelling instance, got %d", count)
	}

	// 3. Process the modification of a specific instance
	modifyInstanceFile, err := os.Open("../test/maildir/cur/test-03-3.eml")
	if err != nil {
		t.Fatalf("Error opening test-03-3.eml: %v", err)
	}
	defer modifyInstanceFile.Close()

	msg, err = processor.ProcessEmail(modifyInstanceFile)
	if err != nil {
		t.Fatalf("Error processing modified instance email: %v", err)
	}
	t.Logf("Modified instance mail processing result: %s", msg)

	// Verify we still have 1 event (same UID, just updated)
	count = store.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event after modifying instance, got %d", count)
	}

	// Get the final event and check it has EXDATE or exception dates/times
	events, err = store.ListEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("Error listing events after all operations: %v", err)
	}

	finalEvent := events[0]
	finalCal, err := ical.DecodeCalendar(finalEvent.RawData)
	if err != nil {
		t.Fatalf("Error decoding final event: %v", err)
	}

	// Check for recurring event exceptions - either EXDATE for cancellations
	// or additional VEVENT components with RECURRENCE-ID for modified instances
	exceptionFound := false
	modificationFound := false

	for _, component := range finalCal.Children {
		if component.Name == "VEVENT" {
			exdate := component.Props.Get("EXDATE")
			if exdate != nil {
				exceptionFound = true
			}

			recurrenceId := component.Props.Get("RECURRENCE-ID")
			if recurrenceId != nil {
				modificationFound = true
			}
		}
	}

	// Either the cancellation should be stored as an EXDATE or
	// we should have additional components with RECURRENCE-ID
	if !exceptionFound && !modificationFound {
		t.Errorf("Expected event to have exception or modification markers after updates")
	}
}
