package ical

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"strings"
	"time"

	goical "github.com/emersion/go-ical"
)

// ParseCalendarData parses calendar data from an email MIME part
func ParseCalendarData(part *multipart.Part) (*Event, error) {
	// Read all data from the part
	body, err := io.ReadAll(part)
	if err != nil {
		return nil, fmt.Errorf("reading calendar data: %w", err)
	}

	// Check if we need to base64 decode
	contentTransferEncoding := part.Header.Get("Content-Transfer-Encoding")
	var calData []byte
	if strings.ToLower(contentTransferEncoding) == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(string(body))
		if err != nil {
			return nil, fmt.Errorf("decoding base64: %w", err)
		}
		calData = decoded
	} else {
		calData = body
	}

	// Parse the calendar data
	event, err := ParseICalData(calData)
	if err != nil {
		return nil, fmt.Errorf("extracting calendar info: %w", err)
	}

	return event, nil
}

// ParseCalendarReader parses calendar data from an io.Reader with optional encoding
func ParseCalendarReader(r io.Reader, encoding string) (*Event, error) {
	// Read all data from the reader
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading calendar data: %w", err)
	}

	// Check if we need to base64 decode
	var calData []byte
	if strings.ToLower(encoding) == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(string(body))
		if err != nil {
			return nil, fmt.Errorf("decoding base64: %w", err)
		}
		calData = decoded
	} else {
		calData = body
	}

	// Parse the calendar data
	event, err := ParseICalData(calData)
	if err != nil {
		return nil, fmt.Errorf("extracting calendar info: %w", err)
	}

	return event, nil
}

// ParseICalData parses iCalendar data and extracts basic event information
func ParseICalData(icsData []byte) (*Event, error) {
	// Use a defer-recover to handle any panics in the decoder
	var cal *goical.Calendar
	var err error
	
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "Warning: Panic in iCal decoder: %v\n", r)
				err = fmt.Errorf("panic in decoder: %v", r)
			}
		}()
		
		cal, err = goical.NewDecoder(bytes.NewReader(icsData)).Decode()
	}()
	
	if err != nil {
		return nil, fmt.Errorf("parsing iCal data: %w", err)
	}

	// Store the raw data
	event := &Event{
		RawData: icsData,
	}

	// Extract METHOD if present at calendar level
	methodProp := cal.Props.Get("METHOD")
	if methodProp != nil {
		event.Method = methodProp.Value
	}

	// Find the first VEVENT component
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		// Extract UID - required by iCalendar standard
		uidProp := component.Props.Get("UID")
		if uidProp != nil {
			event.UID = uidProp.Value
		} else {
			// No UID found - this violates the iCalendar standard
			return nil, fmt.Errorf("VEVENT missing required UID property")
		}

		// Extract Summary (optional)
		summaryProp := component.Props.Get("SUMMARY")
		if summaryProp != nil {
			event.Summary = summaryProp.Value
		} else {
			event.Summary = "Event without summary"
		}

		// Extract SEQUENCE (optional)
		sequenceProp := component.Props.Get("SEQUENCE")
		if sequenceProp != nil {
			// Try to parse the sequence number, default to 0 if invalid
			var seq int
			if _, err := fmt.Sscanf(sequenceProp.Value, "%d", &seq); err == nil {
				event.Sequence = seq
			}
		}

		return event, nil
	}

	// If no VEVENT found, return an error
	return nil, fmt.Errorf("no VEVENT component found in iCalendar data")
}

// DecodeCalendar parses iCalendar data into a Calendar object
func DecodeCalendar(icsData []byte) (*goical.Calendar, error) {
	var cal *goical.Calendar
	var err error
	
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "Warning: Panic in iCal decoder: %v\n", r)
				err = fmt.Errorf("panic in decoder: %v", r)
			}
		}()
		
		cal, err = goical.NewDecoder(bytes.NewReader(icsData)).Decode()
	}()
	
	if err != nil {
		return nil, err
	}
	
	return cal, nil
}

// EncodeCalendar encodes a Calendar object to bytes
func EncodeCalendar(cal *goical.Calendar) ([]byte, error) {
	var buf bytes.Buffer
	encoder := goical.NewEncoder(&buf)
	
	if err := encoder.Encode(cal); err != nil {
		return nil, fmt.Errorf("encoding calendar: %w", err)
	}
	
	return buf.Bytes(), nil
}

// TestEventDecodeAndEncode attempts to decode and re-encode the event to validate
// that it can be properly stored without errors
func TestEventDecodeAndEncode(rawIcalData []byte) error {
	// First attempt to decode the calendar
	cal, err := DecodeCalendar(rawIcalData)
	if err != nil {
		return fmt.Errorf("decoding calendar: %w", err)
	}
	
	// Check for VEVENT components and validate them
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}
		
		// Check for required DTSTAMP property
		if component.Props.Get("DTSTAMP") == nil {
			// Add a DTSTAMP property if missing
			now := time.Now().UTC().Format("20060102T150405Z")
			component.Props.Set(&goical.Prop{Name: "DTSTAMP", Value: now})
		}
		
		// Check for exclusive DTEND and DURATION properties (can't have both)
		if component.Props.Get("DTEND") != nil && component.Props.Get("DURATION") != nil {
			// Remove DURATION if both are present (prioritize DTEND)
			component.Props.Del("DURATION")
		}
	}
	
	// Now try to re-encode to validate
	_, err = EncodeCalendar(cal)
	if err != nil {
		return fmt.Errorf("encoding calendar: %w", err)
	}
	
	return nil
}

// NewCalendar creates a new iCalendar object
func NewCalendar() *goical.Calendar {
	cal := goical.NewCalendar()
	cal.Props.Set(&goical.Prop{Name: "PRODID", Value: "-//calmailproc//Calendar//EN"})
	cal.Props.Set(&goical.Prop{Name: "VERSION", Value: "2.0"})
	return cal
}

// Calendar is the exported type for go-ical.Calendar
type Calendar = goical.Calendar

// Component is the exported type for go-ical.Component
type Component = goical.Component

// Prop is the exported type for go-ical.Prop
type Prop = goical.Prop

