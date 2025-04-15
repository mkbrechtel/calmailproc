package icalfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofrs/flock"
	"github.com/mkbrechtel/calmailproc/parser/ical"
	"github.com/mkbrechtel/calmailproc/storage"
	"github.com/mkbrechtel/calmailproc/storage/memory"
)

// ICalFileStorage implements the storage.Storage interface using a single iCalendar file
// It uses memory storage as a backend for improved performance, loading the file only once
// and writing it back when closed.
type ICalFileStorage struct {
	memory.MemoryStorage
	FilePath  string
	fileLock  sync.RWMutex // Lock for in-process concurrency
	fileFlock *flock.Flock // Filesystem-level lock
	file      *os.File     // File handle for I/O operations
	isOpen    bool
	modified  bool
}

// NewICalFileStorage creates a new ICalFileStorage with the given file path
func NewICalFileStorage(filePath string) (*ICalFileStorage, error) {
	storage := &ICalFileStorage{
		MemoryStorage: *memory.NewMemoryStorage(),
		FilePath:      filePath,
		fileFlock:     flock.New(filePath),
	}

	// Create the directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating directory for calendar file: %w", err)
	}

	return storage, nil
}

// ReadAndLockOpen loads the ical file into memory storage and locks the file
func (s *ICalFileStorage) ReadAndLockOpen() error {
	// Use sync.RWMutex for in-process concurrency
	s.fileLock.Lock()

	if s.isOpen {
		return nil // Already open
	}

	// Acquire exclusive lock on the file
	locked, err := s.fileFlock.TryLock()
	if err != nil {
		s.fileLock.Unlock() // Unlock on error
		return fmt.Errorf("attempting to lock calendar file: %w", err)
	}
	if !locked {
		s.fileLock.Unlock() // Unlock on error
		return fmt.Errorf("calendar file is locked by another process")
	}

	// Open file with read-write access, create if not exists
	s.file, err = os.OpenFile(s.FilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		s.fileFlock.Unlock() // Release the flock
		s.fileLock.Unlock()  // Unlock on error
		return fmt.Errorf("opening calendar file: %w", err)
	}

	// Get file info to check size
	fileInfo, err := s.file.Stat()
	if err != nil {
		s.file.Close()
		s.fileFlock.Unlock() // Release the flock
		s.fileLock.Unlock()  // Unlock on error
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
		s.file.Close()
		s.fileFlock.Unlock() // Release the flock
		s.fileLock.Unlock()  // Unlock on error
		return fmt.Errorf("reading calendar file: %w", err)
	}

	// Parse the calendar
	cal, err := ical.DecodeCalendar(data)
	if err != nil {
		s.file.Close()
		s.fileFlock.Unlock() // Release the flock
		s.fileLock.Unlock()  // Unlock on error
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

// WriteAndUnlock writes the memory storage back to the ical file if modified,
// and releases the file lock
func (s *ICalFileStorage) WriteAndUnlock() error {
	// Don't need to lock here, as we should already have the lock from ReadAndLockOpen
	if !s.isOpen || s.file == nil {
		return nil // Nothing to do
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

		// Truncate file to ensure old content is removed
		if err := s.file.Truncate(0); err != nil {
			return fmt.Errorf("truncating calendar file: %w", err)
		}

		// Seek to beginning of file
		if _, err := s.file.Seek(0, 0); err != nil {
			return fmt.Errorf("seeking to start of file: %w", err)
		}

		// Write to file
		if _, err := s.file.Write(data); err != nil {
			return fmt.Errorf("writing to calendar file: %w", err)
		}

		// Sync data to disk
		if err := s.file.Sync(); err != nil {
			return fmt.Errorf("syncing calendar file: %w", err)
		}
	}

	// Close the file
	if err := s.file.Close(); err != nil {
		return fmt.Errorf("closing calendar file: %w", err)
	}

	// Release filesystem lock
	if err := s.fileFlock.Unlock(); err != nil {
		return fmt.Errorf("unlocking calendar file: %w", err)
	}

	// Release in-process lock
	s.fileLock.Unlock()

	s.file = nil
	s.isOpen = false
	s.modified = false
	return nil
}

// StoreEvent marks the storage as modified after store operation and requires an active lock
func (s *ICalFileStorage) StoreEvent(event *ical.Event) error {
	// We're assuming the file is already locked by ReadAndLockOpen
	if !s.isOpen || s.file == nil {
		return fmt.Errorf("storage not open, call ReadAndLockOpen first")
	}

	err := s.MemoryStorage.StoreEvent(event)
	if err == nil {
		s.modified = true
	}
	return err
}

// GetEvent retrieves an event from memory
func (s *ICalFileStorage) GetEvent(id string) (*ical.Event, error) {
	if !s.isOpen {
		return nil, fmt.Errorf("storage not open, call ReadAndLockOpen first")
	}

	return s.MemoryStorage.GetEvent(id)
}

// ListEvents lists events from memory
func (s *ICalFileStorage) ListEvents() ([]*ical.Event, error) {
	if !s.isOpen {
		return nil, fmt.Errorf("storage not open, call ReadAndLockOpen first")
	}

	return s.MemoryStorage.ListEvents()
}

// DeleteEvent marks the storage as modified after delete operation and requires an active lock
func (s *ICalFileStorage) DeleteEvent(id string) error {
	// We're assuming the file is already locked by ReadAndLockOpen
	if !s.isOpen || s.file == nil {
		return fmt.Errorf("storage not open, call ReadAndLockOpen first")
	}

	err := s.MemoryStorage.DeleteEvent(id)
	if err == nil {
		s.modified = true
	}
	return err
}

// Ensure ICalFileStorage implements storage.Storage
var _ storage.Storage = (*ICalFileStorage)(nil)
