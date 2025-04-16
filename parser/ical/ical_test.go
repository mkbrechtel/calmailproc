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

func TestTestEventDecodeAndEncode(t *testing.T) {
	// Test case 1: Valid iCalendar data with DTSTAMP and DTEND
	validData := []byte(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//hacksw/handcal//NONSGML v1.0//EN
METHOD:REQUEST
BEGIN:VEVENT
UID:test-uid-123
SUMMARY:Valid Test Event
DTSTAMP:20250415T100000Z
DTSTART:20250415T100000Z
DTEND:20250415T110000Z
END:VEVENT
END:VCALENDAR
`)

	// Test case 2: iCalendar data missing DTSTAMP (should be auto-added)
	missingDTSTAMP := []byte(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//hacksw/handcal//NONSGML v1.0//EN
METHOD:REQUEST
BEGIN:VEVENT
UID:test-uid-456
SUMMARY:Missing DTSTAMP Event
DTSTART:20250415T100000Z
DTEND:20250415T110000Z
END:VEVENT
END:VCALENDAR
`)

	// Test case 3: iCalendar data with both DTEND and DURATION (invalid)
	bothDTENDandDURATION := []byte(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//hacksw/handcal//NONSGML v1.0//EN
METHOD:REQUEST
BEGIN:VEVENT
UID:test-uid-789
SUMMARY:Both DTEND and DURATION Event
DTSTAMP:20250415T100000Z
DTSTART:20250415T100000Z
DTEND:20250415T110000Z
DURATION:PT1H
END:VEVENT
END:VCALENDAR
`)

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "Valid iCalendar data",
			data:    validData,
			wantErr: false,
		},
		{
			name:    "Missing DTSTAMP (should auto-fix)",
			data:    missingDTSTAMP,
			wantErr: false,
		},
		{
			name:    "Both DTEND and DURATION (should auto-fix)",
			data:    bothDTENDandDURATION,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TestEventDecodeAndEncode(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestEventDecodeAndEncode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}