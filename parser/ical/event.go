package ical

import (
	"bytes"
	"time"

	goical "github.com/emersion/go-ical"
)

// Event represents calendar event information
type Event struct {
	UID         string
	RawData     []byte // Raw iCalendar data
	Summary     string
	Start       time.Time
	End         time.Time
	Location    string
	Organizer   string
	Description string
	Method      string // Calendar method (REQUEST, REPLY, CANCEL, etc.)
	Sequence    int    // Sequence number for event updates
}

// IsRecurringUpdate checks if an event is a recurring event update
func (e *Event) IsRecurringUpdate() bool {
	// Parse the event data
	cal, err := goical.NewDecoder(bytes.NewReader(e.RawData)).Decode()
	if err != nil {
		return false
	}

	// Check for RECURRENCE-ID in VEVENT components
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		if component.Props.Get("RECURRENCE-ID") != nil {
			return true
		}
	}

	return false
}
