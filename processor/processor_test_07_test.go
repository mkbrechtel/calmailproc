package processor

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/parser/ical"
	"github.com/mkbrechtel/calmailproc/storage"
)

func TestProcessorTest07SequentialUpdates(t *testing.T) {
	// Create an in-memory storage
	store := storage.NewMemoryStorage()
	processor := NewProcessor(store, true)

	// 1. Process the original event (sequence 0)
	originalFile, err := os.Open("../test/maildir/cur/test-07-1.eml")
	if err != nil {
		t.Fatalf("Error opening test-07-1.eml: %v", err)
	}
	defer originalFile.Close()

	msg, err := processor.ProcessEmail(originalFile)
	if err != nil {
		t.Fatalf("Error processing original event email: %v", err)
	}
	t.Logf("Original event mail processing result: %s", msg)

	// Verify one event was stored
	count := store.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event after original mail, got %d", count)
	}

	// Get the event and check it's the original with sequence 0
	events, err := store.ListEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("Error listing events: %v", err)
	}

	originalEvent := events[0]
	
	// Check sequence 0
	originalCal, err := ical.DecodeCalendar(originalEvent.RawData)
	if err != nil {
		t.Fatalf("Error decoding original event: %v", err)
	}

	var originalSequence string
	for _, component := range originalCal.Children {
		if component.Name == "VEVENT" {
			seq := component.Props.Get("SEQUENCE")
			if seq != nil {
				originalSequence = seq.Value
			}
			break
		}
	}

	if originalSequence != "0" {
		t.Errorf("Expected original sequence to be 0, got %s", originalSequence)
	}

	// Store the original date/time for later comparison
	var originalDtstart, originalDtend string
	for _, component := range originalCal.Children {
		if component.Name == "VEVENT" {
			dtstart := component.Props.Get("DTSTART")
			if dtstart != nil {
				originalDtstart = dtstart.Value
			}

			dtend := component.Props.Get("DTEND")
			if dtend != nil {
				originalDtend = dtend.Value
			}
			break
		}
	}

	// 2. Process update with sequence 3 (test-07-2.eml)
	update3File, err := os.Open("../test/maildir/cur/test-07-2.eml")
	if err != nil {
		t.Fatalf("Error opening test-07-2.eml: %v", err)
	}
	defer update3File.Close()

	msg, err = processor.ProcessEmail(update3File)
	if err != nil {
		t.Fatalf("Error processing sequence 3 update email: %v", err)
	}
	t.Logf("Sequence 3 update mail processing result: %s", msg)

	// Verify we still have 1 event (same UID, just updated)
	count = store.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event after sequence 3 update, got %d", count)
	}

	// 3. Process update with sequence 4 (test-07-3.eml)
	update4File, err := os.Open("../test/maildir/cur/test-07-3.eml")
	if err != nil {
		t.Fatalf("Error opening test-07-3.eml: %v", err)
	}
	defer update4File.Close()

	msg, err = processor.ProcessEmail(update4File)
	if err != nil {
		t.Fatalf("Error processing sequence 4 update email: %v", err)
	}
	t.Logf("Sequence 4 update mail processing result: %s", msg)

	// 4. Process update with sequence 2 (test-07-4.eml) - should be ignored as lower than current
	update2File, err := os.Open("../test/maildir/cur/test-07-4.eml")
	if err != nil {
		t.Fatalf("Error opening test-07-4.eml: %v", err)
	}
	defer update2File.Close()

	msg, err = processor.ProcessEmail(update2File)
	if err != nil {
		t.Fatalf("Error processing sequence 2 update email: %v", err)
	}
	t.Logf("Sequence 2 update mail processing result: %s", msg)

	// 5. Process update with sequence 1 (test-07-5.eml) - should also be ignored
	update1File, err := os.Open("../test/maildir/cur/test-07-5.eml")
	if err != nil {
		t.Fatalf("Error opening test-07-5.eml: %v", err)
	}
	defer update1File.Close()

	msg, err = processor.ProcessEmail(update1File)
	if err != nil {
		t.Fatalf("Error processing sequence 1 update email: %v", err)
	}
	t.Logf("Sequence 1 update mail processing result: %s", msg)

	// Get the final event and verify it has the highest sequence (4)
	events, err = store.ListEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("Error listing events after all updates: %v", err)
	}

	finalEvent := events[0]
	finalCal, err := ical.DecodeCalendar(finalEvent.RawData)
	if err != nil {
		t.Fatalf("Error decoding final event: %v", err)
	}

	var finalSequence string
	for _, component := range finalCal.Children {
		if component.Name == "VEVENT" {
			seq := component.Props.Get("SEQUENCE")
			if seq != nil {
				finalSequence = seq.Value
			}
			break
		}
	}

	if finalSequence != "4" {
		t.Errorf("Expected final sequence to be 4, got %s", finalSequence)
	}

	// Verify the date/time has changed from the original
	var finalDtstart, finalDtend string
	for _, component := range finalCal.Children {
		if component.Name == "VEVENT" {
			dtstart := component.Props.Get("DTSTART")
			if dtstart != nil {
				finalDtstart = dtstart.Value
			}

			dtend := component.Props.Get("DTEND")
			if dtend != nil {
				finalDtend = dtend.Value
			}
			break
		}
	}

	if originalDtstart == finalDtstart {
		t.Errorf("Expected date/time to change, but start time remained %s", originalDtstart)
	}

	if originalDtend == finalDtend {
		t.Errorf("Expected date/time to change, but end time remained %s", originalDtend)
	}

	// Now test processing out of order but with correct sequence preference
	// Reset storage
	storeOutOfOrder := storage.NewMemoryStorage()
	processorOutOfOrder := NewProcessor(storeOutOfOrder, true)

	// 1. Process update with sequence 4 first
	update4File, err = os.Open("../test/maildir/cur/test-07-3.eml")
	if err != nil {
		t.Fatalf("Error re-opening test-07-3.eml: %v", err)
	}
	defer update4File.Close()

	_, err = processorOutOfOrder.ProcessEmail(update4File)
	if err != nil {
		t.Fatalf("Error processing sequence 4 update email: %v", err)
	}

	// 2. Process original with sequence 0
	originalFile, err = os.Open("../test/maildir/cur/test-07-1.eml")
	if err != nil {
		t.Fatalf("Error re-opening test-07-1.eml: %v", err)
	}
	defer originalFile.Close()

	_, err = processorOutOfOrder.ProcessEmail(originalFile)
	if err != nil {
		t.Fatalf("Error processing original event email: %v", err)
	}

	// Get the final event from this test
	eventsOutOfOrder, err := storeOutOfOrder.ListEvents()
	if err != nil || len(eventsOutOfOrder) != 1 {
		t.Fatalf("Error listing events for out-of-order test: %v", err)
	}

	finalOutOfOrderEvent := eventsOutOfOrder[0]
	
	// The processor should have kept the sequence 4 event even though it was processed before the original
	finalOutOfOrderCal, err := ical.DecodeCalendar(finalOutOfOrderEvent.RawData)
	if err != nil {
		t.Fatalf("Error decoding final out-of-order event: %v", err)
	}

	var finalOutOfOrderSequence string
	for _, component := range finalOutOfOrderCal.Children {
		if component.Name == "VEVENT" {
			seq := component.Props.Get("SEQUENCE")
			if seq != nil {
				finalOutOfOrderSequence = seq.Value
			}
			break
		}
	}

	if finalOutOfOrderSequence != "4" {
		t.Errorf("Expected out-of-order final sequence to be 4, got %s", finalOutOfOrderSequence)
	}
}
