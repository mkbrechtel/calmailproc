package parser

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
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
			return email, fmt.Errorf("no boundary found for multipart message")
		}

		// Read the multipart body
		mr := multipart.NewReader(msg.Body, boundary)
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return email, fmt.Errorf("reading multipart: %w", err)
			}

			// Check for calendar part
			partContentType := part.Header.Get("Content-Type")
			if strings.Contains(partContentType, "text/calendar") {
				email.HasCalendar = true
				if err := extractCalendarData(part, email); err != nil {
					return email, fmt.Errorf("extracting calendar data: %w", err)
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
	// Use the go-ical package to parse just enough to get the UID
	cal, err := ical.NewDecoder(bytes.NewReader(icsData)).Decode()
	if err != nil {
		return nil, fmt.Errorf("parsing iCal: %w", err)
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
		}

		// Extract Summary (optional)
		summaryProp := component.Props.Get("SUMMARY")
		if summaryProp != nil {
			event.Summary = summaryProp.Value
		}

		return event, nil
	}

	return nil, fmt.Errorf("no VEVENT component found in iCalendar data")
}
