package email

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
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

	// Check if this is a calendar content directly
	if strings.Contains(mediaType, "text/calendar") || strings.Contains(mediaType, "application/ics") {
		email.HasCalendar = true
		transferEncoding := msg.Header.Get("Content-Transfer-Encoding")
		event, err := ical.ParseCalendarReader(msg.Body, transferEncoding)
		if err != nil {
			email.HasCalendar = false
			return email, fmt.Errorf("extracting calendar data: %w", err)
		}
		email.Event = event
	} else if strings.HasPrefix(mediaType, "multipart/") {
		// Handle multipart message
		boundary := params["boundary"]
		if boundary == "" {
			// Continue processing without multipart if boundary is missing
			return email, fmt.Errorf("no boundary found for multipart message")
		}

		// Process the multipart message recursively to handle nested multiparts
		var err error
		email, err = processMultipart(msg.Body, boundary, email)
		if err != nil {
			return email, fmt.Errorf("processing multipart: %w", err)
		}
	}

	return email, nil
}

// processMultipart processes a multipart message part and recursively processes nested multiparts
func processMultipart(r io.Reader, boundary string, email *Email) (*Email, error) {
	mr := multipart.NewReader(r, boundary)
	
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return email, fmt.Errorf("error reading multipart: %w", err)
		}

		// Get the part's content type
		partContentType := part.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(partContentType)
		if err != nil {
			// Skip parts with invalid content type
			continue
		}

		// Check for calendar parts
		contentDisposition := part.Header.Get("Content-Disposition")
		fileName := ""
		if contentDisposition != "" {
			_, params, err := mime.ParseMediaType(contentDisposition)
			if err == nil && params["filename"] != "" {
				fileName = params["filename"]
			}
		}

		if strings.Contains(mediaType, "text/calendar") || 
		   strings.Contains(mediaType, "application/ics") ||
		   strings.Contains(mediaType, "text/x-vCalendar") ||
		   strings.HasSuffix(fileName, ".ics") {
			
			email.HasCalendar = true
			event, err := ical.ParseCalendarData(part)
			if err != nil {
				email.HasCalendar = false
				return email, fmt.Errorf("extracting calendar data: %w", err)
			}
			email.Event = event
			// Continue processing other parts in case there are multiple calendar entries
		} else if strings.HasPrefix(mediaType, "multipart/") {
			// Process nested multipart content recursively
			nestedBoundary := params["boundary"]
			if nestedBoundary != "" {
				// Create a copy of the part data
				partData, err := io.ReadAll(part)
				if err != nil {
					return email, fmt.Errorf("reading nested multipart data: %w", err)
				}
				
				// Process the nested multipart
				var nestedErr error
				email, nestedErr = processMultipart(strings.NewReader(string(partData)), nestedBoundary, email)
				if nestedErr != nil {
					return email, fmt.Errorf("processing nested multipart: %w", nestedErr)
				}
			}
		}
	}

	return email, nil
}