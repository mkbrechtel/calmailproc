package storage

import (
	"github.com/mkbrechtel/calmailproc/parser/ical"
)

// Storage defines the interface for storing calendar events
type Storage interface {
	// StoreEvent stores a calendar event in the storage
	StoreEvent(event *ical.Event) error

	// GetEvent retrieves a calendar event from the storage by its UID
	GetEvent(id string) (*ical.Event, error)

	// ListEvents lists all events in the storage
	ListEvents() ([]*ical.Event, error)

	// DeleteEvent deletes a calendar event from the storage by its UID
	DeleteEvent(id string) error
}
