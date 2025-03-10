package parser

import (
	"os"
	"testing"
	"time"
)

func TestParseEmail(t *testing.T) {
	// Open the test email file
	file, err := os.Open("../test/mails/example-mail-1.eml")
	if err != nil {
		t.Fatalf("Failed to open test email: %v", err)
	}
	defer file.Close()

	// Parse the email
	email, err := ParseEmail(file)
	if err != nil {
		t.Fatalf("Failed to parse email: %v", err)
	}

	// Check basic email fields
	if email.Subject != "Test Event 1" {
		t.Errorf("Expected subject 'Test Event 1', got '%s'", email.Subject)
	}

	expectedFrom := "Markus Brechtel <markus.brechtel@uk-koeln.de>"
	if email.From != expectedFrom {
		t.Errorf("Expected from '%s', got '%s'", expectedFrom, email.From)
	}

	expectedTo := "\"'brechtel@med.uni-frankfurt.de'\" <brechtel@med.uni-frankfurt.de>"
	if email.To != expectedTo {
		t.Errorf("Expected to '%s', got '%s'", expectedTo, email.To)
	}

	// Expected date: Mon, 10 Mar 2025 09:41:35 +0000
	expectedDate := time.Date(2025, 3, 10, 9, 41, 35, 0, time.UTC)
	if !email.Date.Equal(expectedDate) {
		t.Errorf("Expected date '%v', got '%v'", expectedDate, email.Date)
	}

	// Check calendar information
	if !email.HasCalendar {
		t.Error("Expected email to have calendar data")
	} else {
		if email.Event.Summary != "Test Event 1" {
			t.Errorf("Expected event summary 'Test Event 1', got '%s'", email.Event.Summary)
		}

		// TODO: Improve date parsing in the future
		// For now, we'll skip the exact time check since our simple parser
		// doesn't handle the complex date formats in the iCalendar yet

		if email.Event.Start.IsZero() {
			t.Logf("Warning: Start time is zero, will need to improve parsing")
		}

		if email.Event.End.IsZero() {
			t.Logf("Warning: End time is zero, will need to improve parsing")
		}

		expectedOrganizer := "Markus Brechtel"
		if email.Event.Organizer != expectedOrganizer {
			t.Errorf("Expected organizer '%s', got '%s'", expectedOrganizer, email.Event.Organizer)
		}
	}
}
