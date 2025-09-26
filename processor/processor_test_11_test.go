package processor

import (
	"os"
	"testing"

	"github.com/mkbrechtel/calmailproc/parser/ical"
	"github.com/mkbrechtel/calmailproc/storage"
)

func TestProcessorTest11OutOfSequenceMail(t *testing.T) {
	// Create an in-memory storage
	store := storage.NewMemoryStorage()
	processor := NewProcessor(store, true)

	// Read request mail (chronologically was sent first, but we process it after the cancel)
	requestMailFile, err := os.Open("../test/maildir/cur/test-11-2.eml")
	if err != nil {
		t.Fatalf("Error opening test-11-2.eml: %v", err)
	}
	defer requestMailFile.Close()

	// Process the request mail first (which has an earlier timestamp)
	msg, err := processor.ProcessEmail(requestMailFile)
	if err != nil {
		t.Fatalf("Error processing request email: %v", err)
	}
	t.Logf("Request mail processing result: %s", msg)

	// Read cancellation mail (chronologically was sent after, has newer timestamp)
	cancelMailFile, err := os.Open("../test/maildir/cur/test-11-1.eml")
	if err != nil {
		t.Fatalf("Error opening test-11-1.eml: %v", err)
	}
	defer cancelMailFile.Close()

	// Process the cancel mail
	msg, err = processor.ProcessEmail(cancelMailFile)
	if err != nil {
		t.Fatalf("Error processing cancellation email: %v", err)
	}
	t.Logf("Cancel mail processing result: %s", msg)

	// Check the final state: event should be CANCELLED since the cancellation has a later DTSTAMP
	event, err := store.GetEvent("040000008200E00074C5B7101A82E0080000000071F706FA87AFDB010000000000000000100000004BEAA256602AD04DB92DF1261AC49A16")
	if err != nil {
		t.Fatalf("Error retrieving event: %v", err)
	}
	if event == nil {
		t.Fatal("Event not found in storage")
	}

	// Now test reverse order: if we process cancel and then request (maildir order)
	// Test reset
	store = storage.NewMemoryStorage()
	processor = NewProcessor(store, true)

	// Read cancellation mail again
	cancelMailFile, err = os.Open("../test/maildir/cur/test-11-1.eml")
	if err != nil {
		t.Fatalf("Error opening test-11-1.eml: %v", err)
	}
	defer cancelMailFile.Close()

	// Process the cancel mail first
	msg, err = processor.ProcessEmail(cancelMailFile)
	if err != nil {
		t.Fatalf("Error processing cancellation email: %v", err)
	}
	t.Logf("Cancel mail processing result: %s", msg)

	// Read request mail again
	requestMailFile, err = os.Open("../test/maildir/cur/test-11-2.eml")
	if err != nil {
		t.Fatalf("Error opening test-11-2.eml: %v", err)
	}
	defer requestMailFile.Close()

	// Process the request mail second
	msg, err = processor.ProcessEmail(requestMailFile)
	if err != nil {
		t.Fatalf("Error processing request email: %v", err)
	}
	t.Logf("Request mail processing result: %s", msg)

	// Check the final state again: event should still be CANCELLED
	// because the cancel event has a newer DTSTAMP even though processed first
	event, err = store.GetEvent("040000008200E00074C5B7101A82E0080000000071F706FA87AFDB010000000000000000100000004BEAA256602AD04DB92DF1261AC49A16")
	if err != nil {
		t.Fatalf("Error retrieving event: %v", err)
	}
	if event == nil {
		t.Fatal("Event not found in storage")
	}

	// Check if the final state shows CANCELLED
	cal, err := ical.DecodeCalendar(event.RawData)
	if err != nil {
		t.Fatalf("Error decoding event: %v", err)
	}

	var hasStatusCancelled bool
	for _, component := range cal.Children {
		if component.Name == "VEVENT" {
			status := component.Props.Get("STATUS")
			if status != nil && status.Value == "CANCELLED" {
				hasStatusCancelled = true
				break
			}
		}
	}

	if !hasStatusCancelled {
		t.Error("Event should be CANCELLED but it is not")
	}
}

