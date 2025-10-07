package processor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mkbrechtel/calmailproc/parser/ical"
	"github.com/mkbrechtel/calmailproc/storage"
)

// TestInstanceBeforeParent tests the scenario where a recurring event instance update
// arrives before the parent recurring event is received. This verifies that the system
// can handle out-of-order processing correctly.
func TestInstanceBeforeParent(t *testing.T) {
	store := storage.NewMemoryStorage()
	proc := NewProcessor(store, false)

	testPath := "../test/maildir/cur"
	uid := "synthetic-test18-event@example.com"

	// Step 1: Process instance update first (test-18-0.eml)
	t.Run("1. Processing instance update before parent", func(t *testing.T) {
		filePath := filepath.Join(testPath, "test-18-0.eml")
		file, err := os.Open(filePath)
		if err != nil {
			t.Fatalf("Failed to open instance file: %v", err)
		}
		defer file.Close()

		result, err := proc.ProcessEmail(file)
		if err != nil {
			t.Fatalf("Failed to process instance email: %v", err)
		}

		t.Logf("Instance processing result: %s", result)

		if !contains(result, "Stored new event") {
			t.Errorf("Expected instance to be stored as new event, got: %s", result)
		}

		event, err := store.GetEvent(uid)
		if err != nil {
			t.Fatalf("Failed to retrieve stored instance: %v", err)
		}

		if event.Sequence != 2 {
			t.Errorf("Instance should have sequence 2, got: %d", event.Sequence)
		}

		cal, err := ical.DecodeCalendar(event.RawData)
		if err != nil {
			t.Fatalf("Failed to decode calendar: %v", err)
		}

		hasRecurrenceID := false
		for _, component := range cal.Children {
			if component.Name == "VEVENT" {
				if recProp := component.Props.Get("RECURRENCE-ID"); recProp != nil {
					hasRecurrenceID = true
					t.Logf("Instance has RECURRENCE-ID: %s", recProp.Value)
				}
			}
		}

		if !hasRecurrenceID {
			t.Error("Stored event should have RECURRENCE-ID")
		}
	})

	// Step 2: Process parent event (test-18-1.eml)
	t.Run("2. Processing parent event after instance", func(t *testing.T) {
		filePath := filepath.Join(testPath, "test-18-1.eml")
		file, err := os.Open(filePath)
		if err != nil {
			t.Fatalf("Failed to open parent file: %v", err)
		}
		defer file.Close()

		result, err := proc.ProcessEmail(file)
		if err != nil {
			t.Fatalf("Failed to process parent email: %v", err)
		}

		t.Logf("Parent processing result: %s", result)

		if !contains(result, "Updated parent event while preserving instances") {
			t.Errorf("Expected parent to update while preserving instance, got: %s", result)
		}

		event, err := store.GetEvent(uid)
		if err != nil {
			t.Fatalf("Failed to retrieve event after parent processing: %v", err)
		}

		cal, err := ical.DecodeCalendar(event.RawData)
		if err != nil {
			t.Fatalf("Failed to decode calendar: %v", err)
		}

		masterCount := 0
		instanceCount := 0

		for _, component := range cal.Children {
			if component.Name == "VEVENT" {
				if component.Props.Get("RECURRENCE-ID") == nil {
					masterCount++

					rruleProp := component.Props.Get("RRULE")
					if rruleProp == nil {
						t.Error("Master event should have RRULE")
					} else {
						t.Logf("Master has RRULE: %s", rruleProp.Value)
					}

					seqProp := component.Props.Get("SEQUENCE")
					if seqProp != nil {
						t.Logf("Master sequence: %s", seqProp.Value)
					}
				} else {
					instanceCount++

					recProp := component.Props.Get("RECURRENCE-ID")
					seqProp := component.Props.Get("SEQUENCE")

					recID := recProp.Value
					seq := "nil"
					if seqProp != nil {
						seq = seqProp.Value
					}

					t.Logf("Instance: RECURRENCE-ID=%s, SEQUENCE=%s", recID, seq)
				}
			}
		}

		if masterCount != 1 {
			t.Errorf("Expected 1 master event, found %d", masterCount)
		}

		if instanceCount != 1 {
			t.Errorf("Expected 1 instance (the modified one), found %d", instanceCount)
		}

		t.Log("SUCCESS: Parent event correctly merged with existing instance")
	})

	// Step 3: Final verification
	t.Run("3. Final state verification", func(t *testing.T) {
		event, err := store.GetEvent(uid)
		if err != nil {
			t.Fatalf("Failed to retrieve final event: %v", err)
		}

		cal, err := ical.DecodeCalendar(event.RawData)
		if err != nil {
			t.Fatalf("Failed to decode final calendar: %v", err)
		}

		var summary string
		for _, component := range cal.Children {
			if component.Name == "VEVENT" && component.Props.Get("RECURRENCE-ID") == nil {
				if sumProp := component.Props.Get("SUMMARY"); sumProp != nil {
					summary = sumProp.Value
				}
			}
		}

		if summary != "Test 18 - Weekly Meeting" {
			t.Errorf("Expected summary 'Test 18 - Weekly Meeting', got: %s", summary)
		}

		t.Logf("Final calendar contains recurring event with preserved instance exception")
	})
}