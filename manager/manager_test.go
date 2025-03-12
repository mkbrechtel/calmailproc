package manager

import (
	"testing"
	
	goical "github.com/emersion/go-ical"
	"github.com/mkbrechtel/calmailproc/parser/ical"
)

// TestHandleRecurringEventUpdate tests the HandleRecurringEventUpdate function
func TestHandleRecurringEventUpdate(t *testing.T) {
	manager := NewDefaultManager()

	// Create a basic calendar with a master event
	existingCal := ical.NewCalendar()
	masterEvent := goical.NewComponent("VEVENT")
	masterEvent.Props.Set(&goical.Prop{Name: "UID", Value: "event123"})
	masterEvent.Props.Set(&goical.Prop{Name: "SUMMARY", Value: "Test Event"})
	existingCal.Children = append(existingCal.Children, masterEvent)

	// Create a recurring instance update
	recurrenceUpdate := goical.NewComponent("VEVENT")
	recurrenceUpdate.Props.Set(&goical.Prop{Name: "UID", Value: "event123"})
	recurrenceUpdate.Props.Set(&goical.Prop{Name: "RECURRENCE-ID", Value: "20220101T120000Z"})
	recurrenceUpdate.Props.Set(&goical.Prop{Name: "SUMMARY", Value: "Updated Instance"})

	// Test adding a new instance
	updatedCal, err := manager.HandleRecurringEventUpdate(existingCal, recurrenceUpdate, "")
	if err != nil {
		t.Fatalf("HandleRecurringEventUpdate failed: %v", err)
	}

	// Count the VEVENT components in the calendar
	var eventCount int
	for _, component := range updatedCal.Children {
		if component.Name == "VEVENT" {
			eventCount++
		}
	}
	
	// Verify the calendar now has at least two events
	if eventCount < 2 {
		t.Errorf("Expected at least 2 events, got %d", eventCount)
	}

	// Test updating an existing instance
	// First add the instance to the calendar
	existingCal.Children = append(existingCal.Children, recurrenceUpdate)

	// Now update it
	modifiedUpdate := goical.NewComponent("VEVENT")
	modifiedUpdate.Props.Set(&goical.Prop{Name: "UID", Value: "event123"})
	modifiedUpdate.Props.Set(&goical.Prop{Name: "RECURRENCE-ID", Value: "20220101T120000Z"})
	modifiedUpdate.Props.Set(&goical.Prop{Name: "SUMMARY", Value: "Re-updated Instance"})

	updatedCal, err = manager.HandleRecurringEventUpdate(existingCal, modifiedUpdate, "")
	if err != nil {
		t.Fatalf("HandleRecurringEventUpdate failed: %v", err)
	}

	// Count the VEVENT components in the calendar
	eventCount = 0
	for _, component := range updatedCal.Children {
		if component.Name == "VEVENT" {
			eventCount++
		}
	}
	
	// Just verify that the calendar has events
	if eventCount < 2 {
		t.Errorf("Expected at least 2 events, got %d", eventCount)
	}

	// Test cancellation
	cancelUpdate := goical.NewComponent("VEVENT")
	cancelUpdate.Props.Set(&goical.Prop{Name: "UID", Value: "event123"})
	cancelUpdate.Props.Set(&goical.Prop{Name: "RECURRENCE-ID", Value: "20220101T120000Z"})

	updatedCal, err = manager.HandleRecurringEventUpdate(existingCal, cancelUpdate, "CANCEL")
	if err != nil {
		t.Fatalf("HandleRecurringEventUpdate failed: %v", err)
	}

	// Find the cancelled instance and verify its status
	var found bool
	// First get the recurrence instance by UID and RECURRENCE-ID
	for _, component := range updatedCal.Children {
		if component.Name != "VEVENT" {
			continue
		}

		uidProp := component.Props.Get("UID")
		if uidProp == nil || uidProp.Value != "event123" {
			continue
		}
		
		recurrenceID := component.Props.Get("RECURRENCE-ID")
		if recurrenceID != nil && recurrenceID.Value == "20220101T120000Z" {
			found = true
			
			// Check if the component has a STATUS property
			statusProps := component.Props["STATUS"]
			if len(statusProps) == 0 {
				t.Errorf("Expected status CANCELLED, but no STATUS property found")
			} else {
				status := statusProps[0]
				if status.Value != "CANCELLED" {
					t.Errorf("Expected status CANCELLED, got %s", status.Value)
				}
			}
			break
		}
	}

	if !found {
		t.Errorf("Could not find cancelled instance")
	}
}

// TestMatchesRecurrenceID tests the matchesRecurrenceID function
func TestMatchesRecurrenceID(t *testing.T) {
	// Create two events with the same RECURRENCE-ID
	event1 := goical.NewComponent("VEVENT")
	event1.Props.Set(&goical.Prop{Name: "RECURRENCE-ID", Value: "20220101T120000Z"})

	event2 := goical.NewComponent("VEVENT")
	event2.Props.Set(&goical.Prop{Name: "RECURRENCE-ID", Value: "20220101T120000Z"})

	if !matchesRecurrenceID(event1, event2) {
		t.Errorf("Events with same RECURRENCE-ID should match")
	}

	// Create two events with different RECURRENCE-ID
	event3 := goical.NewComponent("VEVENT")
	event3.Props.Set(&goical.Prop{Name: "RECURRENCE-ID", Value: "20220102T120000Z"})

	if matchesRecurrenceID(event1, event3) {
		t.Errorf("Events with different RECURRENCE-ID should not match")
	}

	// Create two events without RECURRENCE-ID (should match)
	event4 := goical.NewComponent("VEVENT")
	event5 := goical.NewComponent("VEVENT")

	if !matchesRecurrenceID(event4, event5) {
		t.Errorf("Events without RECURRENCE-ID should match")
	}

	// Create one event with and one without RECURRENCE-ID (should not match)
	if matchesRecurrenceID(event1, event4) {
		t.Errorf("Events where one has RECURRENCE-ID and one doesn't should not match")
	}
}