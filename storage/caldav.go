package storage

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	goical "github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	icalParser "github.com/mkbrechtel/calmailproc/parser/ical"
)

// CalDAVStorage implements the storage.Storage interface using CalDAV
type CalDAVStorage struct {
	client       *caldav.Client
	calendarPath string
}

// NewCalDAVStorageFromURL creates a new CalDAVStorage from a single URL with embedded credentials
// Format: http://user:pass@host:port/path/to/calendar/
func NewCalDAVStorageFromURL(fullURL string) (*CalDAVStorage, error) {
	// Parse the URL
	u, err := url.Parse(fullURL)
	if err != nil {
		return nil, fmt.Errorf("parsing CalDAV URL: %w", err)
	}

	// Extract credentials
	var username, password string
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}

	if username == "" || password == "" {
		return nil, fmt.Errorf("CalDAV URL must include credentials (e.g., http://user:pass@host:port/calendar/)")
	}

	// Build server URL without credentials
	serverURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	calendarPath := u.Path

	return NewCalDAVStorage(serverURL, username, password, calendarPath)
}

// NewCalDAVStorage creates a new CalDAVStorage with the given server URL
func NewCalDAVStorage(serverURL, username, password, calendarPath string) (*CalDAVStorage, error) {
	// Create HTTP client with basic auth
	httpClient := &http.Client{}
	authClient := webdav.HTTPClientWithBasicAuth(httpClient, username, password)

	// Create CalDAV client
	client, err := caldav.NewClient(authClient, serverURL)
	if err != nil {
		return nil, fmt.Errorf("creating CalDAV client: %w", err)
	}

	// Ensure calendar path starts with /
	if !strings.HasPrefix(calendarPath, "/") {
		calendarPath = "/" + calendarPath
	}

	// Ensure calendar path ends with /
	if !strings.HasSuffix(calendarPath, "/") {
		calendarPath = calendarPath + "/"
	}

	// Use the calendar path directly (it already contains the full path)
	fullCalendarPath := calendarPath

	return &CalDAVStorage{
		client:       client,
		calendarPath: fullCalendarPath,
	}, nil
}

// StoreEvent stores a calendar event via CalDAV
func (s *CalDAVStorage) StoreEvent(event *icalParser.Event) error {
	if event.UID == "" {
		return fmt.Errorf("event has no UID")
	}

	if len(event.RawData) == 0 {
		return fmt.Errorf("no raw calendar data to store")
	}

	// Create the event path
	eventPath := s.calendarPath + event.UID + ".ics"

	// Parse the raw data into an ical.Calendar
	dec := goical.NewDecoder(bytes.NewReader(event.RawData))
	cal, err := dec.Decode()
	if err != nil {
		return fmt.Errorf("parsing calendar data: %w", err)
	}

	// Use PutCalendarObject with the parsed calendar
	ctx := context.Background()
	_, err = s.client.PutCalendarObject(ctx, eventPath, cal)
	if err != nil {
		return fmt.Errorf("storing event via CalDAV: %w", err)
	}

	return nil
}

// GetEvent retrieves a calendar event from CalDAV by its UID
func (s *CalDAVStorage) GetEvent(uid string) (*icalParser.Event, error) {
	// Create the event path
	eventPath := s.calendarPath + uid + ".ics"

	// Create a multiget request for the specific event
	req := &caldav.CalendarMultiGet{
		Paths: []string{eventPath},
		CompRequest: caldav.CalendarCompRequest{
			AllProps: true,
			AllComps: true,
		},
	}

	// Execute the multiget request
	ctx := context.Background()
	objects, err := s.client.MultiGetCalendar(ctx, s.calendarPath, req)
	if err != nil {
		return nil, fmt.Errorf("getting event from CalDAV: %w", err)
	}

	if len(objects) == 0 {
		return nil, fmt.Errorf("event not found")
	}

	// Get the raw calendar data by encoding it back
	obj := objects[0]
	var buf bytes.Buffer
	enc := goical.NewEncoder(&buf)
	if err := enc.Encode(obj.Data); err != nil {
		return nil, fmt.Errorf("encoding calendar data: %w", err)
	}
	rawData := buf.Bytes()

	// Parse the event to get structured data
	parsedEvent := &icalParser.Event{
		UID:     uid,
		RawData: rawData,
	}

	return parsedEvent, nil
}

// ListEvents lists all events from the CalDAV calendar
func (s *CalDAVStorage) ListEvents() ([]*icalParser.Event, error) {
	// Create a calendar query to get all events
	query := &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{
			Name:     "VCALENDAR",
			AllProps: true,
			AllComps: true,
		},
		CompFilter: caldav.CompFilter{
			Name: "VCALENDAR",
			Comps: []caldav.CompFilter{
				{
					Name: "VEVENT",
				},
			},
		},
	}

	// Execute the query
	ctx := context.Background()
	objects, err := s.client.QueryCalendar(ctx, s.calendarPath, query)
	if err != nil {
		return nil, fmt.Errorf("querying CalDAV calendar: %w", err)
	}

	// Convert CalendarObjects to Events
	events := make([]*icalParser.Event, 0, len(objects))
	for _, obj := range objects {
		// Encode the calendar back to raw data
		var buf bytes.Buffer
		enc := goical.NewEncoder(&buf)
		if err := enc.Encode(obj.Data); err != nil {
			continue // Skip this event if we can't encode it
		}
		
		// Extract UID from the calendar data
		var uid string
		for _, comp := range obj.Data.Children {
			if comp.Name == goical.CompEvent {
				if uidProp := comp.Props.Get(goical.PropUID); uidProp != nil {
					uid = uidProp.Value
					break
				}
			}
		}
		
		if uid == "" {
			// Fallback to extracting from path if UID not found in data
			uid = strings.TrimSuffix(strings.TrimPrefix(obj.Path, s.calendarPath), ".ics")
		}
		
		event := &icalParser.Event{
			UID:     uid,
			RawData: buf.Bytes(),
		}
		events = append(events, event)
	}

	return events, nil
}

// DeleteEvent deletes a calendar event from CalDAV by its UID
func (s *CalDAVStorage) DeleteEvent(uid string) error {
	// Create the event path
	eventPath := s.calendarPath + uid + ".ics"

	// Delete the event
	ctx := context.Background()
	err := s.client.RemoveAll(ctx, eventPath)
	if err != nil {
		return fmt.Errorf("deleting event from CalDAV: %w", err)
	}

	return nil
}