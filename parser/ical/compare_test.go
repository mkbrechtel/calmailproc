package ical

import (
	"testing"
	"time"
)

func TestCompareEvents(t *testing.T) {
	tests := []struct {
		name     string
		event1   *Event
		event2   *Event
		expected ComparisonResult
	}{
		{
			name:     "nil events",
			event1:   nil,
			event2:   nil,
			expected: EventsEqual,
		},
		{
			name:     "first event nil",
			event1:   nil,
			event2:   &Event{Sequence: 1},
			expected: SecondEventNewer,
		},
		{
			name:     "second event nil",
			event1:   &Event{Sequence: 1},
			event2:   nil,
			expected: FirstEventNewer,
		},
		{
			name: "higher sequence wins",
			event1: &Event{
				Sequence: 2,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTAMP:20250417T110049Z\r\nDTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			event2: &Event{
				Sequence: 1,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTAMP:20250417T112140Z\r\nDTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			expected: FirstEventNewer,
		},
		{
			name: "lower sequence loses",
			event1: &Event{
				Sequence: 0,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTAMP:20250417T112140Z\r\nDTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			event2: &Event{
				Sequence: 1,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTAMP:20250417T110049Z\r\nDTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			expected: SecondEventNewer,
		},
		{
			name: "same sequence, newer timestamp wins",
			event1: &Event{
				Sequence: 0,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTAMP:20250417T112140Z\r\nDTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			event2: &Event{
				Sequence: 0,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTAMP:20250417T110049Z\r\nDTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			expected: FirstEventNewer,
		},
		{
			name: "same sequence, older timestamp loses",
			event1: &Event{
				Sequence: 0,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTAMP:20250417T110049Z\r\nDTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			event2: &Event{
				Sequence: 0,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTAMP:20250417T112140Z\r\nDTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			expected: SecondEventNewer,
		},
		{
			name: "test case 11: cancel with same sequence but newer timestamp",
			event1: &Event{
				Method:   "CANCEL",
				Sequence: 0,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nMETHOD:CANCEL\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTAMP:20250417T112140Z\r\nDTSTART:20250418T080000Z\r\nSTATUS:CANCELLED\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			event2: &Event{
				Method:   "REQUEST",
				Sequence: 0,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nMETHOD:REQUEST\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTAMP:20250417T110049Z\r\nDTSTART:20250418T080000Z\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			expected: FirstEventNewer,
		},
		{
			name: "equal events",
			event1: &Event{
				Sequence: 1,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTAMP:20250417T110049Z\r\nDTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			event2: &Event{
				Sequence: 1,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTAMP:20250417T110049Z\r\nDTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			expected: EventsEqual,
		},
		{
			name: "missing DTSTAMP in first event",
			event1: &Event{
				Sequence: 1,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			event2: &Event{
				Sequence: 1,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTAMP:20250417T110049Z\r\nDTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			expected: SecondEventNewer,
		},
		{
			name: "missing DTSTAMP in second event",
			event1: &Event{
				Sequence: 1,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTAMP:20250417T110049Z\r\nDTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			event2: &Event{
				Sequence: 1,
				RawData:  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//hacksw/handcal//NONSGML v1.0//EN\r\nBEGIN:VEVENT\r\nUID:test-event\r\nDTSTART:20250418T080000Z\r\nEND:VEVENT\r\nEND:VCALENDAR"),
			},
			expected: FirstEventNewer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CompareEvents(tt.event1, tt.event2)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseICalTime(t *testing.T) {
	tests := []struct {
		name     string
		timeStr  string
		expected time.Time
		isError  bool
	}{
		{
			name:     "UTC timestamp with Z",
			timeStr:  "20250417T112140Z",
			expected: time.Date(2025, 4, 17, 11, 21, 40, 0, time.UTC),
			isError:  false,
		},
		{
			name:     "local timestamp without Z",
			timeStr:  "20250417T112140",
			expected: time.Date(2025, 4, 17, 11, 21, 40, 0, time.UTC),
			isError:  false,
		},
		{
			name:     "ISO format with Z",
			timeStr:  "2023-07-01T122600Z",
			expected: time.Date(2023, 7, 1, 12, 26, 0, 0, time.UTC),
			isError:  false,
		},
		{
			name:     "ISO format without Z",
			timeStr:  "2023-07-01T122600",
			expected: time.Date(2023, 7, 1, 12, 26, 0, 0, time.UTC),
			isError:  false,
		},
		{
			name:     "invalid format",
			timeStr:  "2025-04-17T11:21:40:123Z",
			expected: time.Time{},
			isError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseICalTime(tt.timeStr)
			if tt.isError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.isError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.isError && !result.Equal(tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}