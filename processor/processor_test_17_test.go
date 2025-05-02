package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mkbrechtel/calmailproc/parser/email"
	"github.com/mkbrechtel/calmailproc/parser/ical"
)

// TestExtractRecurrenceIDs extracts RECURRENCE-ID and sequence information from test files
func TestExtractRecurrenceIDs(t *testing.T) {
	// Path to test emails
	testPath := "../test/maildir/cur"

	// Process all test-17 emails
	for i := 0; i <= 7; i++ {
		fileName := fmt.Sprintf("test-17-%d.eml", i)
		filePath := filepath.Join(testPath, fileName)
		
		file, err := os.Open(filePath)
		if err != nil {
			t.Fatalf("Failed to open file %s: %v", fileName, err)
		}

		// Parse the email
		parsedEmail, err := email.Parse(file)
		file.Close()
		if err != nil {
			t.Fatalf("Failed to parse email %s: %v", fileName, err)
		}

		// Check if it has a calendar event
		if parsedEmail.HasCalendar && parsedEmail.Event.UID != "" {
			// Extract calendar data
			cal, err := ical.DecodeCalendar(parsedEmail.Event.RawData)
			if err != nil {
				t.Fatalf("Failed to decode calendar in %s: %v", fileName, err)
			}

			// Find RECURRENCE-ID if present
			recurrenceID := "N/A (Parent Event)"
			for _, component := range cal.Children {
				if component.Name != "VEVENT" {
					continue
				}

				if recProp := component.Props.Get("RECURRENCE-ID"); recProp != nil {
					recurrenceID = recProp.Value
					break
				}
			}

			// Print information about the event
			t.Logf("Email %s: IsRecurringUpdate=%v, Sequence=%d, RecurrenceID=%s", 
				fileName, 
				parsedEmail.Event.IsRecurringUpdate(), 
				parsedEmail.Event.Sequence,
				recurrenceID)
		} else {
			t.Logf("Email %s: No calendar event found", fileName)
		}
	}
}