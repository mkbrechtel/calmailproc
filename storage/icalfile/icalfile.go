package icalfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mkbrechtel/calmailproc/parser/ical"
	"github.com/mkbrechtel/calmailproc/storage/memory"
)

// ICalFileStorage implements the storage.Storage interface using a single iCalendar file
// It uses memory storage as a backend for improved performance, loading the file only once
// and writing it back when closed.
type ICalFileStorage struct {
	memory.MemoryStorage
	FilePath string
	fileLock sync.RWMutex // Lock for file operations
	isOpen   bool
	modified bool
}

// NewICalFileStorage creates a new ICalFileStorage with the given file path
func NewICalFileStorage(filePath string) (*ICalFileStorage, error) {
	storage := &ICalFileStorage{
		MemoryStorage: *memory.NewMemoryStorage(),
		FilePath:      filePath,
	}

	// Create the directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating directory for calendar file: %w", err)
	}

	return storage, nil
}

// Open loads the ical file into memory storage
func (s *ICalFileStorage) OpenAndLock() error {
	s.fileLock.Lock()
	defer s.fileLock.Unlock()

	if s.isOpen {
		return nil // Already open
	}

	// Check if file exists
	fileInfo, err := os.Stat(s.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, nothing to load
			s.isOpen = true
			return nil
		}
		return fmt.Errorf("checking calendar file: %w", err)
	}

	// Don't try to read empty files
	if fileInfo.Size() == 0 {
		s.isOpen = true
		return nil
	}

	// Read the calendar file
	data, err := os.ReadFile(s.FilePath)
	if err != nil {
		return fmt.Errorf("reading calendar file: %w", err)
	}

	// Parse the calendar
	cal, err := ical.DecodeCalendar(data)
	if err != nil {
		return fmt.Errorf("parsing calendar file: %w", err)
	}

	// Clear any existing data in memory storage
	s.MemoryStorage.Clear()

	// Load all events into memory storage
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		uidProp := component.Props.Get("UID")
		if uidProp == nil {
			continue // Skip events without UID
		}

		// Create a minimal calendar containing just this event
		eventCal := ical.NewCalendar()
		eventCal.Children = append(eventCal.Children, component)

		// Encode to raw bytes
		rawData, err := ical.EncodeCalendar(eventCal)
		if err != nil {
			continue // Skip events that can't be encoded
		}

		// Create the event
		event, err := ical.ParseICalData(rawData)
		if err != nil {
			continue // Skip events that can't be parsed
		}

		// Store in memory
		s.MemoryStorage.StoreEvent(event)
	}

	s.isOpen = true
	s.modified = false
	return nil
}

// CloseAndWrite writes the memory storage back to the ical file if modified
func (s *ICalFileStorage) WriteAndUnlock() error {
	s.fileLock.Lock()
	defer s.fileLock.Unlock()

	if !s.isOpen {
		return nil // Already closed
	}

	if s.modified {
		// Get all events from memory
		events, err := s.MemoryStorage.ListEvents()
		if err != nil {
			return fmt.Errorf("listing events from memory: %w", err)
		}

		// Create a new calendar
		cal := ical.NewCalendar()

		// Add all events to the calendar
		for _, event := range events {
			// Parse the event data to extract VEVENT components
			eventCal, err := ical.DecodeCalendar(event.RawData)
			if err != nil {
				continue // Skip events that can't be decoded
			}

			// Extract the VEVENT components
			for _, component := range eventCal.Children {
				if component.Name == "VEVENT" {
					cal.Children = append(cal.Children, component)
				}
			}
		}

		// Encode the calendar
		data, err := ical.EncodeCalendar(cal)
		if err != nil {
			return fmt.Errorf("encoding calendar: %w", err)
		}

		// Write to file
		if err := os.WriteFile(s.FilePath, data, 0644); err != nil {
			return fmt.Errorf("writing calendar file: %w", err)
		}
	}

	s.isOpen = false
	s.modified = false
	return nil
}

// StoreEvent marks the storage as modified after store operation
func (s *ICalFileStorage) StoreEvent(event *ical.Event) error {
	s.fileLock.Lock()
	defer s.fileLock.Unlock()

	if !s.isOpen {
		if err := s.OpenAndLock(); err != nil {
			return err
		}
	}

	err := s.MemoryStorage.StoreEvent(event)
	if err == nil {
		s.modified = true
	}
	return err
}

// DeleteEvent marks the storage as modified after delete operation
func (s *ICalFileStorage) DeleteEvent(id string) error {
	s.fileLock.Lock()
	defer s.fileLock.Unlock()

	if !s.isOpen {
		if err := s.OpenAndLock(); err != nil {
			return err
		}
	}

	err := s.MemoryStorage.DeleteEvent(id)
	if err == nil {
		s.modified = true
	}
	return err
}