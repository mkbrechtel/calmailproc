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
		{
			name:           "Basic calendar invitation",
			emailFile:      "../test/mails/example-mail-01.eml",
			processReplies: true,
			wantStoreCount: 1,
			wantErr:        false,
		},
		{
			name:           "Email with calendar data",
			emailFile:      "../test/mails/example-mail-03.eml",
			processReplies: true,
			wantStoreCount: 1, // Assuming this is a different event
			wantErr:        false,
		},
		{
			name:           "Calendar update",
			emailFile:      "../test/mails/example-mail-05.eml",
			processReplies: true,
			wantStoreCount: 1, // New event
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
	// Find a suitable invitation and reply pair from the existing emails
	// Let's first examine what emails we have available
	
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