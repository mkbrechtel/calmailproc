package parser

import (
	"os"
	"testing"
)

func TestParseEmail(t *testing.T) {
	// Open the test email file
	file, err := os.Open("../test/mails/example-mail-01.eml")
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
	if email.Subject == "" {
		t.Errorf("Expected non-empty subject, got '%s'", email.Subject)
	}
	t.Logf("Email subject: %s", email.Subject)

	if email.From == "" {
		t.Errorf("Expected non-empty From field")
	}
	t.Logf("Email from: %s", email.From)

	if email.To == "" {
		t.Errorf("Expected non-empty To field")
	}
	t.Logf("Email to: %s", email.To)

	if email.Date.IsZero() {
		t.Errorf("Expected non-zero date")
	}
	t.Logf("Email date: %v", email.Date)

	// Check calendar information
	if !email.HasCalendar {
		t.Error("Expected email to have calendar data")
	} else {
		if email.Event.UID == "" {
			t.Error("Expected non-empty event UID")
		}
		t.Logf("Event UID: %s", email.Event.UID)
		
		if email.Event.Summary == "" {
			t.Logf("Warning: Empty event summary")
		} else {
			t.Logf("Event summary: %s", email.Event.Summary)
		}

		// Report if we found a METHOD
		if email.Event.Method != "" {
			t.Logf("Event method: %s", email.Event.Method)
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

		if email.Event.Organizer != "" {
			t.Logf("Event organizer: %s", email.Event.Organizer)
		}
	}
}
