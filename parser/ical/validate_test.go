package ical

import (
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
