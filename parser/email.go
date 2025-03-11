package parser

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// Email represents a parsed email with calendar data if available
type Email struct {
	Subject     string
	From        string
	To          string
	Date        time.Time
	HasCalendar bool
	Event       CalendarEvent
}

// CalendarEvent represents calendar event information
type CalendarEvent struct {
	UID         string
	RawData     []byte // Raw iCalendar data
	Summary     string
	Start       time.Time
	End         time.Time
	Location    string
	Organizer   string
	Description string
	Method      string // Calendar method (REQUEST, REPLY, CANCEL, etc.)
}

// ParseEmail parses an email from an io.Reader and extracts calendar data if present
func ParseEmail(r io.Reader) (*Email, error) {
	// Use Go's standard mail package to parse the email
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return nil, fmt.Errorf("reading message: %w", err)
	}

	// Create the email struct
	email := &Email{
		Subject: msg.Header.Get("Subject"),
		From:    msg.Header.Get("From"),
		To:      msg.Header.Get("To"),
	}

	// Parse the date
	if dateStr := msg.Header.Get("Date"); dateStr != "" {
		if date, err := mail.ParseDate(dateStr); err == nil {
			email.Date = date
		}
	}

	// Get the Content-Type header to determine if this is a multipart message
	contentType := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// If Content-Type parsing fails, just treat the body as text
		return email, nil
	}

	// Check if this is a multipart message
	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			// Continue processing without multipart if boundary is missing
			fmt.Fprintf(os.Stderr, "Warning: No boundary found for multipart message, continuing with basic text\n")
			return email, nil
		}

		// Read the multipart body
		mr := multipart.NewReader(msg.Body, boundary)
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				// Continue processing if one part fails
				fmt.Fprintf(os.Stderr, "Warning: Error reading multipart: %v, continuing with parsed parts\n", err)
				break
			}

			// Check for calendar part
			partContentType := part.Header.Get("Content-Type")
			if strings.Contains(partContentType, "text/calendar") {
				email.HasCalendar = true
				if err := extractCalendarData(part, email); err != nil {
					// Continue without calendar data if extraction fails
					fmt.Fprintf(os.Stderr, "Warning: Error extracting calendar data: %v, continuing without calendar\n", err)
					email.HasCalendar = false
				}
			}
		}
	}

	return email, nil
}

func extractCalendarData(part *multipart.Part, email *Email) error {
	// Read all data from the part
	body, err := io.ReadAll(part)
	if err != nil {
		return fmt.Errorf("reading calendar data: %w", err)
	}

	// Check if we need to base64 decode
	contentTransferEncoding := part.Header.Get("Content-Transfer-Encoding")
	var calData []byte
	if strings.ToLower(contentTransferEncoding) == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(string(body))
		if err != nil {
			return fmt.Errorf("decoding base64: %w", err)
		}
		calData = decoded
	} else {
		calData = body
	}

	// Parse just enough to get the UID and basic info
	event, err := extractBasicCalendarInfo(calData)
	if err != nil {
		return fmt.Errorf("extracting calendar info: %w", err)
	}

	email.Event = *event
	return nil
}

func extractBasicCalendarInfo(icsData []byte) (*CalendarEvent, error) {
	// Create a recovery event in case of panic
	recoveryEvent := &CalendarEvent{
		RawData: icsData,
		UID:     "recovered-uid-" + time.Now().Format("20060102-150405"),
		Summary: "Recovered Calendar Data",
	}

	// Use a defer-recover to handle any panics in the decoder
	var cal *ical.Calendar
	var err error
	
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "Warning: Panic in iCal decoder: %v, using recovery event\n", r)
				err = fmt.Errorf("panic in decoder: %v", r)
			}
		}()
		
		cal, err = ical.NewDecoder(bytes.NewReader(icsData)).Decode()
	}()
	
	if err != nil {
		// Return a minimal event object with the raw data when parsing fails
		fmt.Fprintf(os.Stderr, "Warning: Error parsing iCal data: %v, saving raw data only\n", err)
		return recoveryEvent, nil
	}

	// Store the raw data
	event := &CalendarEvent{
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

		// Extract UID
		uidProp := component.Props.Get("UID")
		if uidProp != nil {
			event.UID = uidProp.Value
		} else {
			// Generate a fallback UID if none found
			event.UID = "generated-uid-" + time.Now().Format("20060102-150405")
		}

		// Extract Summary (optional)
		summaryProp := component.Props.Get("SUMMARY")
		if summaryProp != nil {
			event.Summary = summaryProp.Value
		} else {
			event.Summary = "Event without summary"
		}

		return event, nil
	}

	// If no VEVENT found, create a minimal event
	event.UID = "no-vevent-" + time.Now().Format("20060102-150405")
	event.Summary = "Calendar data without VEVENT"
	
	return event, nil
}
