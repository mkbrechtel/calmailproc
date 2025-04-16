package ical

import (
	"fmt"
)

// TestEventDecodeAndEncode attempts to decode and re-encode the event to validate
// that it can be properly stored without errors
func ValidateEvent(rawIcalData []byte) error {
	// First attempt to decode the calendar
	cal, err := DecodeCalendar(rawIcalData)
	if err != nil {
		return fmt.Errorf("decoding calendar: %w", err)
	}

	// Now try to re-encode to validate
	_, err = EncodeCalendar(cal)
	if err != nil {
		return fmt.Errorf("encoding calendar: %w", err)
	}

	return nil
}
