package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mkbrechtel/calmailproc/parser/ical"
	"github.com/mkbrechtel/calmailproc/storage/memory"
)

// TestTest17Files directly uses the actual test-17 email files to reproduce the bug
func TestTest17Files(t *testing.T) {
	// Create processor with in-memory storage
	store := memory.NewMemoryStorage()
	proc := NewProcessor(store, false)

	// Path to test emails
	testPath := "../test/maildir/cur"

	// Process test-17-0.eml first (parent update with sequence 7)
	parentFile := filepath.Join(testPath, "test-17-0.eml")
	parentReader, err := os.Open(parentFile)
	if err != nil {
		t.Fatalf("Failed to open parent file %s: %v", parentFile, err)
	}
	defer parentReader.Close()

	parentResult, err := proc.ProcessEmail(parentReader)
	if err != nil {
		t.Fatalf("Failed to process parent email: %v", err)
	}
	t.Logf("Parent result (test-17-0.eml): %s", parentResult)

	// Now process all instance update files (test-17-1.eml through test-17-7.eml)
	instanceResults := make([]string, 7)
	skippedCount := 0

	for i := 1; i <= 7; i++ {
		instanceFile := filepath.Join(testPath, fmt.Sprintf("test-17-%d.eml", i))
		instanceReader, err := os.Open(instanceFile)
		if err != nil {
			t.Fatalf("Failed to open instance file %s: %v", instanceFile, err)
		}

		result, err := proc.ProcessEmail(instanceReader)
		instanceReader.Close()
		if err != nil {
			t.Fatalf("Failed to process instance %d email: %v", i, err)
		}

		instanceResults[i-1] = result
		t.Logf("Instance result (test-17-%d.eml): %s", i, result)

		// Check if the instance was skipped
		if strings.Contains(result, "Not processing older event") {
			skippedCount++
		}
	}

	// Test should FAIL if instances are being skipped (which is the bug)
	if skippedCount > 0 {
		t.Errorf("BUG DETECTED: %d of 7 instance updates were incorrectly skipped", skippedCount)
	} else {
		t.Logf("No instances were skipped")
	}

	// Get the final state of the calendar
	eventUID := "040000008200E00074C5B7101A82E0080000000060FA38123DBBDB010000000000000000100000009123BEADE9978A4AA0AC92EF2005A108"
	event, err := store.GetEvent(eventUID)
	if err != nil {
		t.Fatalf("Failed to retrieve event: %v", err)
	}

	// Parse the calendar
	cal, err := ical.DecodeCalendar(event.RawData)
	if err != nil {
		t.Fatalf("Failed to parse calendar data: %v", err)
	}

	// Count instance exceptions with RECURRENCE-ID
	instanceCount := 0
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}
		if component.Props.Get("RECURRENCE-ID") != nil {
			instanceCount++
			recurrenceID := component.Props.Get("RECURRENCE-ID").Value
			t.Logf("Found instance with RECURRENCE-ID=%s", recurrenceID)
		}
	}

	// Test should FAIL if instances were not preserved
	// The current implementation incorrectly loses instances
	if instanceCount == 0 {
		t.Errorf("BUG DETECTED: No instance exceptions found in the final calendar")
	} else if instanceCount < 7 {
		t.Errorf("BUG DETECTED: Only %d instance exceptions found, expected 7", instanceCount)
	} else {
		t.Logf("All %d instance exceptions were correctly preserved", instanceCount)
	}
}

// TestTest17FilesReverseOrder processes the actual test-17 email files in reverse order
// (instances first, then parent) to see how the system should behave
func TestTest17FilesReverseOrder(t *testing.T) {
	// Create processor with in-memory storage
	store := memory.NewMemoryStorage()
	proc := NewProcessor(store, false)

	// Path to test emails
	testPath := "../test/maildir/cur"

	// First process all instance update files (test-17-1.eml through test-17-7.eml)
	for i := 1; i <= 7; i++ {
		instanceFile := filepath.Join(testPath, fmt.Sprintf("test-17-%d.eml", i))
		instanceReader, err := os.Open(instanceFile)
		if err != nil {
			t.Fatalf("Failed to open instance file %s: %v", instanceFile, err)
		}

		result, err := proc.ProcessEmail(instanceReader)
		instanceReader.Close()
		if err != nil {
			t.Fatalf("Failed to process instance %d email: %v", i, err)
		}

		t.Logf("Instance result (test-17-%d.eml): %s", i, result)
	}

	// Check intermediate state before parent update
	eventUID := "040000008200E00074C5B7101A82E0080000000060FA38123DBBDB010000000000000000100000009123BEADE9978A4AA0AC92EF2005A108"
	beforeEvent, err := store.GetEvent(eventUID)
	if err != nil {
		t.Fatalf("Failed to retrieve event before parent update: %v", err)
	}

	beforeCal, err := ical.DecodeCalendar(beforeEvent.RawData)
	if err != nil {
		t.Fatalf("Failed to parse before-parent calendar data: %v", err)
	}

	beforeInstanceCount := 0
	for _, component := range beforeCal.Children {
		if component.Name == "VEVENT" && component.Props.Get("RECURRENCE-ID") != nil {
			beforeInstanceCount++
		}
	}
	t.Logf("Before parent update: calendar has %d instance exceptions", beforeInstanceCount)

	// Now process parent (test-17-0.eml)
	parentFile := filepath.Join(testPath, "test-17-0.eml")
	parentReader, err := os.Open(parentFile)
	if err != nil {
		t.Fatalf("Failed to open parent file %s: %v", parentFile, err)
	}
	defer parentReader.Close()

	parentResult, err := proc.ProcessEmail(parentReader)
	if err != nil {
		t.Fatalf("Failed to process parent email: %v", err)
	}
	t.Logf("Parent result (test-17-0.eml): %s", parentResult)

	// Get the final state after parent update
	afterEvent, err := store.GetEvent(eventUID)
	if err != nil {
		t.Fatalf("Failed to retrieve event after parent update: %v", err)
	}

	afterCal, err := ical.DecodeCalendar(afterEvent.RawData)
	if err != nil {
		t.Fatalf("Failed to parse after-parent calendar data: %v", err)
	}

	// Count instance exceptions after parent update
	afterInstanceCount := 0
	for _, component := range afterCal.Children {
		if component.Name == "VEVENT" && component.Props.Get("RECURRENCE-ID") != nil {
			afterInstanceCount++
			recurrenceID := component.Props.Get("RECURRENCE-ID").Value
			t.Logf("After parent: found instance with RECURRENCE-ID=%s", recurrenceID)
		}
	}

	// Document the current behavior (we should have at least 7 instances)
	// If there are fewer than 7 instances after parent update, that's a bug
	if afterInstanceCount < 7 {
		if beforeInstanceCount > afterInstanceCount {
			t.Logf("CURRENT BEHAVIOR: Parent update caused loss of instances (%d → %d)", 
				beforeInstanceCount, afterInstanceCount)
		}
		t.Errorf("BUG DETECTED: Calendar should have 7 instance exceptions but has only %d", 
			afterInstanceCount)
	} else {
		t.Logf("All 7 instance exceptions were correctly preserved")
	}
}