package email

import (
	"os"
	"testing"
)

func TestParseEmail(t *testing.T) {
	// Open the test email file
	file, err := os.Open("../../test/maildir/cur/example-mail-01.eml")
	if err != nil {
		t.Fatalf("Failed to open test email: %v", err)
	}
	defer file.Close()

	// Parse the email
	email, err := Parse(file)
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
	
	// Check the source description
	if email.SourceDescription != "test-file" {
		t.Errorf("Expected source description 'test-file', got '%s'", email.SourceDescription)
	}
	t.Logf("Source description: %s", email.SourceDescription)

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

		// Check start and end times (should now be properly parsed)
		if email.Event.Start.IsZero() {
			t.Errorf("Expected non-zero start time")
		} else {
			t.Logf("Event start: %v", email.Event.Start.Format("2006-01-02 15:04:05"))
		}

		if email.Event.End.IsZero() {
			t.Errorf("Expected non-zero end time")
		} else {
			t.Logf("Event end: %v", email.Event.End.Format("2006-01-02 15:04:05"))
		}

		if email.Event.Organizer != "" {
			t.Logf("Event organizer: %s", email.Event.Organizer)
		}
	}
}