package ical

import (
	"strings"
	"testing"
)

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
			name:    "Missing DTSTAMP",
			data:    missingDTSTAMP,
			wantErr: true,
		},
		{
			name:    "Both DTEND and DURATION",
			data:    bothDTENDandDURATION,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEvent(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestEventDecodeAndEncode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUID(t *testing.T) {
	tests := []struct {
		name    string
		uid     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Valid UID with alphanumeric",
			uid:     "abc123",
			wantErr: false,
		},
		{
			name:    "Valid UID with email format",
			uid:     "user@example.com",
			wantErr: false,
		},
		{
			name:    "Valid UID with dashes and underscores",
			uid:     "event-123_test",
			wantErr: false,
		},
		{
			name:    "Valid UID with dots and colons",
			uid:     "event.123:test",
			wantErr: false,
		},
		{
			name:    "Valid UID with plus",
			uid:     "event+123",
			wantErr: false,
		},
		{
			name:    "Empty UID",
			uid:     "",
			wantErr: true,
			errMsg:  "UID cannot be empty",
		},
		{
			name:    "UID with 255 characters",
			uid:     strings.Repeat("a", 255),
			wantErr: false,
		},
		{
			name:    "UID exceeding 255 characters",
			uid:     strings.Repeat("a", 256),
			wantErr: true,
			errMsg:  "exceeds maximum length",
		},
		{
			name:    "UID with line break",
			uid:     "event\n123",
			wantErr: true,
			errMsg:  "line breaks",
		},
		{
			name:    "UID with carriage return",
			uid:     "event\r123",
			wantErr: true,
			errMsg:  "line breaks",
		},
		{
			name:    "UID with control character",
			uid:     "event\x00123",
			wantErr: true,
			errMsg:  "control characters",
		},
		{
			name:    "UID with forward slash",
			uid:     "event/123",
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name:    "UID with space",
			uid:     "event 123",
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name:    "UID with special characters",
			uid:     "event#123!",
			wantErr: true,
			errMsg:  "invalid characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUID(tt.uid)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateUID() error message = %v, want message containing %v", err.Error(), tt.errMsg)
			}
		})
	}
}
