package vdir

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mkbrechtel/calmailproc/parser/ical"
)

// VDirStorage implements the storage.Storage interface using the vdir format
type VDirStorage struct {
	BasePath string
}

// NewVDirStorage creates a new VDirStorage with the given base path
func NewVDirStorage(basePath string) (*VDirStorage, error) {
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("creating storage directory: %w", err)
	}

	return &VDirStorage{
		BasePath: basePath,
	}, nil
}

// StoreEvent stores a calendar event in the vdir format
func (s *VDirStorage) StoreEvent(event *ical.Event) error {
	if event.UID == "" {
		return fmt.Errorf("event has no UID")
	}

	if len(event.RawData) == 0 {
		return fmt.Errorf("no raw calendar data to store")
	}

	// Create the event file with a hashed filename and .ics extension
	hashedUID := HashFilename(event.UID)
	filename := fmt.Sprintf("%s.ics", hashedUID)
	filePath := filepath.Join(s.BasePath, filename)

	// Simply write the raw data to the file, replacing any existing file
	if err := os.WriteFile(filePath, event.RawData, 0644); err != nil {
		return fmt.Errorf("writing calendar file: %w", err)
	}

	return nil
}

// GetEvent retrieves a calendar event from the vdir storage
func (s *VDirStorage) GetEvent(id string) (*ical.Event, error) {
	// Find the event file using the hashed ID
	hashedID := HashFilename(id)
	filename := fmt.Sprintf("%s.ics", hashedID)
	filePath := filepath.Join(s.BasePath, filename)

	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading event file: %w", err)
	}

	// Parse the raw data to create an event object
	event, err := ical.ParseICalData(data)
	if err != nil {
		// If parsing fails, create a basic event with just the raw data
		return &ical.Event{
			UID:     id,
			RawData: data,
			Summary: "Unparseable Calendar Event",
		}, nil
	}

	return event, nil
}

// ListEvents lists all events in the vdir storage
func (s *VDirStorage) ListEvents() ([]*ical.Event, error) {
	// List all .ics files in the directory
	entries, err := os.ReadDir(s.BasePath)
	if err != nil {
		return nil, fmt.Errorf("reading storage directory: %w", err)
	}

	var events []*ical.Event
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ics") {
			continue
		}

		// Read the file
		filePath := filepath.Join(s.BasePath, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			// Skip files that can't be read
			continue
		}

		// Parse the data to extract event information
		event, err := ical.ParseICalData(data)
		if err != nil {
			// Skip files that can't be parsed
			continue
		}

		events = append(events, event)
	}

	return events, nil
}

// DeleteEvent deletes a calendar event from the vdir storage
func (s *VDirStorage) DeleteEvent(id string) error {
	// Find the event file using the hashed ID
	hashedID := HashFilename(id)
	filename := fmt.Sprintf("%s.ics", hashedID)
	filePath := filepath.Join(s.BasePath, filename)

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("deleting event file: %w", err)
	}

	return nil
}