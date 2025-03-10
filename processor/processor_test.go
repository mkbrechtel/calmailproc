package processor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mkbrechtel/calmailproc/storage/memory"
)

func TestProcessEmail(t *testing.T) {
	// Create a new memory storage for testing
	memStorage := memory.NewMemoryStorage()

	tests := []struct {
		name           string
		emailFile      string
		processReplies bool
		wantStoreCount int
		wantErr        bool
	}{
		// Test all mail files individually
		{
			name:           "Example mail 01 - Basic calendar invitation",
			emailFile:      "../test/mails/example-mail-01.eml",
			processReplies: true,
			wantStoreCount: 1,
			wantErr:        false,
		},
		{
			name:           "Example mail 02 - Cancelled event",
			emailFile:      "../test/mails/example-mail-02.eml",
			processReplies: true,
			wantStoreCount: 1,
			wantErr:        false,
		},
		{
			name:           "Example mail 03 - Calendar event",
			emailFile:      "../test/mails/example-mail-03.eml",
			processReplies: true,
			wantStoreCount: 1,
			wantErr:        false,
		},
		{
			name:           "Example mail 04 - Calendar event",
			emailFile:      "../test/mails/example-mail-04.eml",
			processReplies: true,
			wantStoreCount: 1,
			wantErr:        false,
		},
		{
			name:           "Example mail 05 - Original recurring event",
			emailFile:      "../test/mails/example-mail-05.eml",
			processReplies: true,
			wantStoreCount: 1,
			wantErr:        false,
		},
		{
			name:           "Example mail 06 - Cancelled instance",
			emailFile:      "../test/mails/example-mail-06.eml",
			processReplies: true,
			wantStoreCount: 1,
			wantErr:        false,
		},
		{
			name:           "Example mail 07 - Modified instance",
			emailFile:      "../test/mails/example-mail-07.eml",
			processReplies: true,
			wantStoreCount: 1,
			wantErr:        false,
		},
		{
			name:           "Example mail 08 - No calendar data",
			emailFile:      "../test/mails/example-mail-08.eml",
			processReplies: true,
			wantStoreCount: 0, // No calendar data to store
			wantErr:        false,
		},
		{
			name:           "Example mail 09 - No calendar data",
			emailFile:      "../test/mails/example-mail-09.eml",
			processReplies: true,
			wantStoreCount: 0, // No calendar data to store
			wantErr:        false,
		},
		{
			name:           "Example mail 10 - No calendar data",
			emailFile:      "../test/mails/example-mail-10.eml",
			processReplies: true,
			wantStoreCount: 0, // No calendar data to store
			wantErr:        false,
		},
		{
			name:           "Example mail 11 - No calendar data",
			emailFile:      "../test/mails/example-mail-11.eml",
			processReplies: true,
			wantStoreCount: 0, // No calendar data to store
			wantErr:        false,
		},
		{
			name:           "Example mail 12 with replies=true - Reply",
			emailFile:      "../test/mails/example-mail-12.eml",
			processReplies: true,
			wantStoreCount: 1, // Reply should be stored
			wantErr:        false,
		},
		{
			name:           "Example mail 12 with replies=false - Reply",
			emailFile:      "../test/mails/example-mail-12.eml",
			processReplies: false,
			wantStoreCount: 0, // Reply should be ignored
			wantErr:        false,
		},
		{
			name:           "Example mail 13 - No calendar data",
			emailFile:      "../test/mails/example-mail-13.eml",
			processReplies: true,
			wantStoreCount: 0, // No calendar data to store
			wantErr:        false,
		},
		{
			name:           "Example mail 14 - Reply with replies=true",
			emailFile:      "../test/mails/example-mail-14.eml",
			processReplies: true,
			wantStoreCount: 1, // Reply should be stored
			wantErr:        false,
		},
		{
			name:           "Example mail 14 with replies=false - Reply",
			emailFile:      "../test/mails/example-mail-14.eml",
			processReplies: false,
			wantStoreCount: 0, // Reply should be ignored
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset the storage for each test
			memStorage.Clear()

			// Create a processor with the specified processReplies setting
			proc := NewProcessor(memStorage, tt.processReplies)

			// Open the email file
			file, err := os.Open(tt.emailFile)
			if err != nil {
				t.Fatalf("Failed to open email file: %v", err)
			}
			defer file.Close()

			// Process the email
			err = proc.ProcessEmail(file, false, true)

			// Check for errors
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessEmail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check the storage count
			if count := memStorage.GetEventCount(); count != tt.wantStoreCount {
				t.Errorf("ProcessEmail() stored %d events, want %d", count, tt.wantStoreCount)
			}
		})
	}
}

func TestProcessAllEmails(t *testing.T) {
	// Create a memory storage
	memStorage := memory.NewMemoryStorage()

	// Create a processor
	proc := NewProcessor(memStorage, true)

	// Process all example emails
	mailsDir := "../test/mails"
	files, err := os.ReadDir(mailsDir)
	if err != nil {
		t.Fatalf("Failed to read mails directory: %v", err)
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".eml" {
			continue
		}

		filePath := filepath.Join(mailsDir, file.Name())
		emailFile, err := os.Open(filePath)
		if err != nil {
			t.Fatalf("Failed to open email file %s: %v", filePath, err)
		}

		err = proc.ProcessEmail(emailFile, false, true)
		emailFile.Close()

		if err != nil {
			t.Errorf("Failed to process email %s: %v", filePath, err)
		}
	}

	// Verify we have stored some events (just a basic sanity check)
	eventCount := memStorage.GetEventCount()
	if eventCount == 0 {
		t.Errorf("No events were stored after processing all emails")
	}

	t.Logf("Successfully processed all emails and stored %d events", eventCount)

	// List and log all stored events for verification
	events, err := memStorage.ListEvents()
	if err != nil {
		t.Fatalf("Failed to list events: %v", err)
	}

	for _, event := range events {
		t.Logf("Stored event - UID: %s, Summary: %s, Method: %s",
			event.UID, event.Summary, event.Method)
	}
}

func TestProcessRepliesControl(t *testing.T) {
	// Test with process-replies=true
	memStorage := memory.NewMemoryStorage()
	procWithReplies := NewProcessor(memStorage, true)

	// Process a REPLY type email with process-replies=true (should be stored)
	replyFile, err := os.Open("../test/mails/example-mail-12.eml")
	if err != nil {
		t.Fatalf("Failed to open reply file: %v", err)
	}

	err = procWithReplies.ProcessEmail(replyFile, false, true)
	replyFile.Close()
	if err != nil {
		t.Fatalf("Failed to process reply with process-replies=true: %v", err)
	}

	// Check that the reply was stored
	withRepliesCount := memStorage.GetEventCount()
	if withRepliesCount == 0 {
		t.Errorf("Reply was not stored with process-replies=true")
	}

	// Test with process-replies=false
	memStorage.Clear()
	procWithoutReplies := NewProcessor(memStorage, false)

	// Process same REPLY email with process-replies=false (should be ignored)
	replyFile, err = os.Open("../test/mails/example-mail-12.eml")
	if err != nil {
		t.Fatalf("Failed to open reply file: %v", err)
	}

	err = procWithoutReplies.ProcessEmail(replyFile, false, true)
	replyFile.Close()
	if err != nil {
		t.Fatalf("Failed to process reply with process-replies=false: %v", err)
	}

	// Check that the reply was ignored
	withoutRepliesCount := memStorage.GetEventCount()
	if withoutRepliesCount > 0 {
		t.Errorf("Reply was stored with process-replies=false")
	}
}

func TestRecurringEventSequence(t *testing.T) {
	// Test the sequence of events for a recurring event series:
	// 1. Original event (05)
	// 2. Modification to an instance (07)
	// 3. Cancellation of an instance (06)
	memStorage := memory.NewMemoryStorage()
	proc := NewProcessor(memStorage, true)

	// Step 1: Process the original recurring event
	originalFile, err := os.Open("../test/mails/example-mail-05.eml")
	if err != nil {
		t.Fatalf("Failed to open original event file: %v", err)
	}
	err = proc.ProcessEmail(originalFile, false, true)
	originalFile.Close()
	if err != nil {
		t.Fatalf("Failed to process original event: %v", err)
	}

	// Verify we have 1 event
	count := memStorage.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event after original, got %d", count)
	}

	// Step 2: Process the modification to a specific instance
	modifiedFile, err := os.Open("../test/mails/example-mail-07.eml")
	if err != nil {
		t.Fatalf("Failed to open modified event file: %v", err)
	}
	err = proc.ProcessEmail(modifiedFile, false, true)
	modifiedFile.Close()
	if err != nil {
		t.Fatalf("Failed to process modified event: %v", err)
	}

	// Verify we still have 1 event (same UID, just updated)
	count = memStorage.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event after modification, got %d", count)
	}

	// Retrieve the event and check it
	events, err := memStorage.ListEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("Failed to list events or wrong count: %v", err)
	}

	event := events[0]
	// The recurring event UID from the test files
	recurringUID := "040000008200E00074C5B7101A82E0080000000044440AFCBB91DB0100000000000000001000000087598F58784D4541BAA76F1829CFE9A1"
	if event.UID != recurringUID {
		t.Errorf("Wrong event UID: %s, expected %s", event.UID, recurringUID)
	}

	// Step 3: Process the cancellation of a specific instance
	cancelFile, err := os.Open("../test/mails/example-mail-06.eml")
	if err != nil {
		t.Fatalf("Failed to open cancelled event file: %v", err)
	}
	err = proc.ProcessEmail(cancelFile, false, true)
	cancelFile.Close()
	if err != nil {
		t.Fatalf("Failed to process cancelled event: %v", err)
	}

	// Verify we still have 1 event (same UID, just updated)
	count = memStorage.GetEventCount()
	if count != 1 {
		t.Errorf("Expected 1 event after cancellation, got %d", count)
	}

	// Retrieve the event again to check cancellation was processed
	events, err = memStorage.ListEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("Failed to list events or wrong count after cancellation: %v", err)
	}

	event = events[0]
	// Verify still the same UID
	if event.UID != recurringUID {
		t.Errorf("Wrong event UID after cancellation: %s", event.UID)
	}
}
