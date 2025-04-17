package ical

import (
	"bytes"
	"fmt"
	"time"

	goical "github.com/emersion/go-ical"
)

// ComparisonResult indicates the result of comparing two calendar events
type ComparisonResult int

const (
	// FirstEventNewer indicates the first event is newer than the second
	FirstEventNewer ComparisonResult = 1
	// SecondEventNewer indicates the second event is newer than the first
	SecondEventNewer ComparisonResult = -1
	// EventsEqual indicates the events have the same recency
	EventsEqual ComparisonResult = 0
)

// CompareEvents compares two calendar events to determine which one is more recent
// and should take precedence. The comparison is based on:
// 1. Sequence number (higher wins)
// 2. DTSTAMP (more recent wins) if sequence numbers are equal
//
// Returns:
//   - FirstEventNewer (1) if event1 should take precedence
//   - SecondEventNewer (-1) if event2 should take precedence
//   - EventsEqual (0) if they have equal precedence
func CompareEvents(event1, event2 *Event) (ComparisonResult, error) {
	// If one of the events is nil, the other takes precedence
	if event1 == nil && event2 == nil {
		return EventsEqual, nil
	}
	if event1 == nil {
		return SecondEventNewer, nil
	}
	if event2 == nil {
		return FirstEventNewer, nil
	}

	// Compare sequence numbers first
	if event1.Sequence > event2.Sequence {
		return FirstEventNewer, nil
	}
	if event1.Sequence < event2.Sequence {
		return SecondEventNewer, nil
	}

	// If sequence numbers are equal, check DTSTAMP
	dtstamp1, err1 := extractDTSTAMP(event1.RawData)
	dtstamp2, err2 := extractDTSTAMP(event2.RawData)

	// Handle errors in parsing DTSTAMP
	if err1 != nil && err2 != nil {
		return EventsEqual, fmt.Errorf("failed to extract DTSTAMP from both events: %v, %v", err1, err2)
	}
	if err1 != nil {
		return SecondEventNewer, nil
	}
	if err2 != nil {
		return FirstEventNewer, nil
	}

	// Compare DTSTAMP values
	if dtstamp1.After(dtstamp2) {
		return FirstEventNewer, nil
	}
	if dtstamp2.After(dtstamp1) {
		return SecondEventNewer, nil
	}

	// If everything is equal, they have the same precedence
	return EventsEqual, nil
}

// extractDTSTAMP extracts the DTSTAMP property from the iCalendar data
func extractDTSTAMP(icsData []byte) (time.Time, error) {
	var cal *goical.Calendar
	var err error

	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in decoder: %v", r)
			}
		}()
		
		cal, err = goical.NewDecoder(bytes.NewReader(icsData)).Decode()
	}()
	
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing iCal data for DTSTAMP: %w", err)
	}

	// Find the first VEVENT component
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		dtstampProp := component.Props.Get("DTSTAMP")
		if dtstampProp == nil {
			return time.Time{}, fmt.Errorf("VEVENT missing DTSTAMP property")
		}

		// Parse the timestamp in iCalendar format
		return parseICalTime(dtstampProp.Value)
	}

	return time.Time{}, fmt.Errorf("no VEVENT component found")
}

// parseICalTime parses an iCalendar timestamp string into a time.Time
func parseICalTime(timeStr string) (time.Time, error) {
	// First try standard iCalendar format: 20250417T112140Z (yyyyMMddTHHmmssZ)
	layout := "20060102T150405Z"
	
	// Handle timestamps with timezone identifier
	if len(timeStr) > 0 && timeStr[len(timeStr)-1] != 'Z' {
		// For simplicity, assume UTC if no Z suffix
		// A more complete solution would handle all iCalendar datetime formats
		layout = "20060102T150405"
	}

	t, err := time.Parse(layout, timeStr)
	if err == nil {
		return t, nil
	}

	// Fallback to ISO format if standard iCalendar format fails
	// Try ISO format: 2023-07-01T122600Z
	isoLayout := "2006-01-02T150405Z"
	if len(timeStr) > 0 && timeStr[len(timeStr)-1] != 'Z' {
		isoLayout = "2006-01-02T150405"
	}

	return time.Parse(isoLayout, timeStr)
}
