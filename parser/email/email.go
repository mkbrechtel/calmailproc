package email

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/mkbrechtel/calmailproc/parser/ical"
)

// Email represents a parsed email with calendar data if available
type Email struct {
	Subject           string
	From              string
	To                string
	Date              time.Time
	HasCalendar       bool
	Event             *ical.Event
	SourceDescription string // Description of the email source (filename, stdin, etc.)
}

// Parse parses an email from an io.Reader and extracts calendar data if present
func Parse(r io.Reader) (*Email, error) {
	// Use Go's standard mail package to parse the email
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return nil, fmt.Errorf("reading message: %w", err)
	}

	// Create the email struct
	email := &Email{
		Subject:           msg.Header.Get("Subject"),
		From:              msg.Header.Get("From"),
		To:                msg.Header.Get("To"),
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
				event, err := ical.ParseCalendarData(part)
				if err != nil {
					// Continue without calendar data if extraction fails
					fmt.Fprintf(os.Stderr, "Warning: Error extracting calendar data: %v, continuing without event\n", err)
					email.HasCalendar = false
				} else {
					email.Event = event
				}
			}
		}
	}

	return email, nil
}