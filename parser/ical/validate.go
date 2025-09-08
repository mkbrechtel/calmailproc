package ical

import (
	"fmt"
	"regexp"
	"strings"
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

// ValidateUID validates that a UID contains only valid characters and is within length limits.
// Common restrictions:
// - Maximum length of 255 characters (common CalDAV limitation)
// - Should contain only printable ASCII characters
// - Should not contain control characters or line breaks
func ValidateUID(uid string) error {
	if uid == "" {
		return fmt.Errorf("UID cannot be empty")
	}
	
	// Check maximum length (255 is a common CalDAV server limit)
	if len(uid) > 255 {
		return fmt.Errorf("UID exceeds maximum length of 255 characters (length: %d)", len(uid))
	}
	
	// Check for control characters, line breaks, and other invalid characters
	// Allow printable ASCII characters plus some common special characters used in UIDs
	// This includes: letters, digits, and common UID characters like @, -, _, ., :, +
	validUIDPattern := regexp.MustCompile(`^[a-zA-Z0-9@\-_.:+]+$`)
	if !validUIDPattern.MatchString(uid) {
		// Check for specific problematic characters to provide better error messages
		if strings.ContainsAny(uid, "\r\n") {
			return fmt.Errorf("UID contains line breaks which are not allowed")
		}
		if strings.ContainsAny(uid, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0b\x0c\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f") {
			return fmt.Errorf("UID contains control characters which are not allowed")
		}
		return fmt.Errorf("UID contains invalid characters (only alphanumeric and @-_.:+ allowed)")
	}
	
	return nil
}
