package ical

import (
	"bytes"
	"mime/multipart"
	"net/textproto"
	"testing"
)

func TestParseICalData(t *testing.T) {
	// Simple iCalendar data for testing
	icsData := []byte(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//hacksw/handcal//NONSGML v1.0//EN
METHOD:REQUEST
BEGIN:VEVENT
UID:test-uid-123
SUMMARY:Test Event
SEQUENCE:0
END:VEVENT
END:VCALENDAR
`)

	// Parse the data
	event, err := ParseICalData(icsData)
	if err != nil {
		t.Fatalf("Failed to parse iCalendar data: %v", err)
	}

	// Check basic fields
	if event.UID != "test-uid-123" {
		t.Errorf("Expected UID 'test-uid-123', got '%s'", event.UID)
	}

	if event.Summary != "Test Event" {
		t.Errorf("Expected summary 'Test Event', got '%s'", event.Summary)
	}

	if event.Method != "REQUEST" {
		t.Errorf("Expected method 'REQUEST', got '%s'", event.Method)
	}

	if event.Sequence != 0 {
		t.Errorf("Expected sequence 0, got %d", event.Sequence)
	}
}

func TestParseCalendarData(t *testing.T) {
	// Create a test MIME part with calendar data
	icsData := []byte(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//hacksw/handcal//NONSGML v1.0//EN
METHOD:REQUEST
BEGIN:VEVENT
UID:test-uid-456
SUMMARY:Test MIME Event
SEQUENCE:1
END:VEVENT
END:VCALENDAR
`)

	// Create a MIME part
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Type", "text/calendar; charset=UTF-8")
	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatalf("Failed to create MIME part: %v", err)
	}
	part.Write(icsData)
	writer.Close()

	// Parse the MIME message
	reader := multipart.NewReader(bytes.NewReader(buf.Bytes()), writer.Boundary())
	mimePart, err := reader.NextPart()
	if err != nil {
		t.Fatalf("Failed to read MIME part: %v", err)
	}

	// Parse the calendar data
	event, err := ParseCalendarData(mimePart)
	if err != nil {
		t.Fatalf("Failed to parse calendar data from MIME part: %v", err)
	}

	// Check basic fields
	if event.UID != "test-uid-456" {
		t.Errorf("Expected UID 'test-uid-456', got '%s'", event.UID)
	}

	if event.Summary != "Test MIME Event" {
		t.Errorf("Expected summary 'Test MIME Event', got '%s'", event.Summary)
	}

	if event.Sequence != 1 {
		t.Errorf("Expected sequence 1, got %d", event.Sequence)
	}
}