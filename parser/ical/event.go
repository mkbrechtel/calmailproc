package ical

import (
	"time"
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