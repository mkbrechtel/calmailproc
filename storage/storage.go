package storage

import (
	"github.com/mkbrechtel/calmailproc/parser"
)

// Storage defines the interface for storing calendar events
type Storage interface {
	// StoreEvent stores a calendar event in the storage
	StoreEvent(event *parser.CalendarEvent) error

	// GetEvent retrieves a calendar event from the storage by its UID
	GetEvent(id string) (*parser.CalendarEvent, error)

	// ListEvents lists all events in the storage
	ListEvents() ([]*parser.CalendarEvent, error)

	// DeleteEvent deletes a calendar event from the storage by its UID
	DeleteEvent(id string) error
}
