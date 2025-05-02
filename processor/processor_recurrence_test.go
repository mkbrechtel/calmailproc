package processor

import (
	"bytes"
	"fmt"
	"testing"

	goical "github.com/emersion/go-ical"
	"github.com/mkbrechtel/calmailproc/parser/ical"
	"github.com/mkbrechtel/calmailproc/storage/memory"
)

// TestProcessEmail_RecurringEvent tests processing recurring event updates
func TestProcessEmail_RecurringEvent(t *testing.T) {
	// Create processor with in-memory storage
	store := memory.NewMemoryStorage()
	proc := NewProcessor(store, true)

	// Store a base recurring event first
	baseEmail := `From: organizer@example.com
To: attendee@example.com
Subject: Meeting invitation
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="boundary"

--boundary
Content-Type: text/plain

Hello, please join my recurring meeting.

--boundary
Content-Type: text/calendar; method=REQUEST; charset=UTF-8
Content-Transfer-Encoding: 7bit

BEGIN:VCALENDAR
PRODID:-//Test//EN
VERSION:2.0
METHOD:REQUEST
BEGIN:VEVENT
SUMMARY:Weekly Meeting
DTSTART:20230101T120000Z
DTEND:20230101T130000Z
UID:recurring-event-1
SEQUENCE:0
RRULE:FREQ=WEEKLY;COUNT=4
ORGANIZER:organizer@example.com
ATTENDEE;PARTSTAT=NEEDS-ACTION:attendee@example.com
DTSTAMP:20230101T000000Z
END:VEVENT
END:VCALENDAR

--boundary--
`

	// Process the original invitation
	_, err := proc.ProcessEmail(bytes.NewBufferString(baseEmail))
	if err != nil {
		t.Fatalf("Failed to process base recurring email: %v", err)
	}

	// Send an update for a specific instance
	instanceEmail := `From: organizer@example.com
To: attendee@example.com
Subject: Updated Meeting
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="boundary"

--boundary
Content-Type: text/plain

The second meeting is rescheduled.

--boundary
Content-Type: text/calendar; method=REQUEST; charset=UTF-8
Content-Transfer-Encoding: 7bit

BEGIN:VCALENDAR
PRODID:-//Test//EN
VERSION:2.0
METHOD:REQUEST
BEGIN:VEVENT
SUMMARY:Weekly Meeting (RESCHEDULED)
DTSTART:20230108T140000Z
DTEND:20230108T150000Z
UID:recurring-event-1
SEQUENCE:1
RECURRENCE-ID:20230108T120000Z
ORGANIZER:organizer@example.com
ATTENDEE;PARTSTAT=NEEDS-ACTION:attendee@example.com
DTSTAMP:20230101T000000Z
END:VEVENT
END:VCALENDAR

--boundary--
`

	// Process the instance update
	_, err = proc.ProcessEmail(bytes.NewBufferString(instanceEmail))
	if err != nil {
		t.Fatalf("Failed to process instance update email: %v", err)
	}

	// Verify that the event was stored and contains both the base event and the update
	event, err := store.GetEvent("recurring-event-1")
	if err != nil {
		t.Fatalf("Failed to retrieve event: %v", err)
	}

	// Parse the calendar to check its contents
	cal, err := ical.DecodeCalendar(event.RawData)
	if err != nil {
		t.Fatalf("Failed to parse calendar data: %v", err)
	}

	// Count the event components - should be at least 2 (base event + instance)
	var eventCount int
	var foundBase, foundInstance bool
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}
		eventCount++

		// Check for the base event (no RECURRENCE-ID)
		if component.Props.Get("RECURRENCE-ID") == nil {
			foundBase = true
			// Verify it still has the original time
			dtstart := component.Props.Get("DTSTART")
			if dtstart != nil && dtstart.Value == "20230101T120000Z" {
				foundBase = true
			}
		}

		// Check for the instance update
		recurrenceID := component.Props.Get("RECURRENCE-ID")
		if recurrenceID != nil && recurrenceID.Value == "20230108T120000Z" {
			foundInstance = true
			// Verify it has the updated time
			dtstart := component.Props.Get("DTSTART")
			if dtstart == nil || dtstart.Value != "20230108T140000Z" {
				t.Errorf("Instance update doesn't have the correct start time")
			}
			
			// Verify it has the updated summary
			summary := component.Props.Get("SUMMARY")
			if summary == nil || summary.Value != "Weekly Meeting (RESCHEDULED)" {
				t.Errorf("Instance update doesn't have the correct summary")
			}
		}
	}

	if eventCount < 2 {
		t.Errorf("Expected at least 2 VEVENT components, got %d", eventCount)
	}

	if !foundBase {
		t.Errorf("Base recurring event not found in the calendar")
	}

	if !foundInstance {
		t.Errorf("Instance update not found in the calendar")
	}
}

// TestMultipleRecurringInstances tests storing an event with multiple items
func TestMultipleRecurringInstances(t *testing.T) {
	// Create a calendar with a base event and multiple instances
	baseEvent := ical.NewCalendar()
	baseEvent.Props.Set(&ical.Prop{Name: "METHOD", Value: "REQUEST"})

	// Add the base event
	master := goical.NewComponent("VEVENT")
	master.Props.Set(&goical.Prop{Name: "UID", Value: "multi-recurring-event"})
	master.Props.Set(&goical.Prop{Name: "SUMMARY", Value: "Multi-Instance Meeting"})
	master.Props.Set(&goical.Prop{Name: "DTSTART", Value: "20230101T120000Z"})
	master.Props.Set(&goical.Prop{Name: "DTEND", Value: "20230101T130000Z"})
	master.Props.Set(&goical.Prop{Name: "RRULE", Value: "FREQ=WEEKLY;COUNT=4"})
	master.Props.Set(&goical.Prop{Name: "SEQUENCE", Value: "0"})
	master.Props.Set(&goical.Prop{Name: "DTSTAMP", Value: "20230101T000000Z"})
	baseEvent.Children = append(baseEvent.Children, master)

	// Add a modified instance
	instance1 := goical.NewComponent("VEVENT")
	instance1.Props.Set(&goical.Prop{Name: "UID", Value: "multi-recurring-event"})
	instance1.Props.Set(&goical.Prop{Name: "SUMMARY", Value: "Modified Instance 1"})
	instance1.Props.Set(&goical.Prop{Name: "DTSTART", Value: "20230108T140000Z"})
	instance1.Props.Set(&goical.Prop{Name: "DTEND", Value: "20230108T150000Z"})
	instance1.Props.Set(&goical.Prop{Name: "RECURRENCE-ID", Value: "20230108T120000Z"})
	instance1.Props.Set(&goical.Prop{Name: "SEQUENCE", Value: "1"})
	instance1.Props.Set(&goical.Prop{Name: "DTSTAMP", Value: "20230101T000000Z"})
	baseEvent.Children = append(baseEvent.Children, instance1)

	// Add another modified instance
	instance2 := goical.NewComponent("VEVENT")
	instance2.Props.Set(&goical.Prop{Name: "UID", Value: "multi-recurring-event"})
	instance2.Props.Set(&goical.Prop{Name: "SUMMARY", Value: "Modified Instance 2"})
	instance2.Props.Set(&goical.Prop{Name: "DTSTART", Value: "20230115T160000Z"})
	instance2.Props.Set(&goical.Prop{Name: "DTEND", Value: "20230115T170000Z"})
	instance2.Props.Set(&goical.Prop{Name: "RECURRENCE-ID", Value: "20230115T120000Z"})
	instance2.Props.Set(&goical.Prop{Name: "SEQUENCE", Value: "1"})
	instance2.Props.Set(&goical.Prop{Name: "DTSTAMP", Value: "20230101T000000Z"})
	baseEvent.Children = append(baseEvent.Children, instance2)

	// Encode the calendar
	calBytes, err := ical.EncodeCalendar(baseEvent)
	if err != nil {
		t.Fatalf("Failed to encode calendar: %v", err)
	}

	// Create the event
	event := &ical.Event{
		UID:      "multi-recurring-event",
		RawData:  calBytes,
		Summary:  "Multi-Instance Meeting",
		Method:   "REQUEST",
		Sequence: 1,
	}

	// Store the event
	store := memory.NewMemoryStorage()
	err = store.StoreEvent(event)
	if err != nil {
		t.Fatalf("Failed to store multi-instance event: %v", err)
	}

	// Retrieve the event
	retrievedEvent, err := store.GetEvent("multi-recurring-event")
	if err != nil {
		t.Fatalf("Failed to retrieve multi-instance event: %v", err)
	}

	// Parse the calendar to check its contents
	cal, err := ical.DecodeCalendar(retrievedEvent.RawData)
	if err != nil {
		t.Fatalf("Failed to parse calendar data: %v", err)
	}

	// Count the event components - should be 3 (base + 2 instances)
	var eventCount int
	var foundBase, foundInstance1, foundInstance2 bool
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}
		eventCount++

		// Check for the base event (no RECURRENCE-ID)
		if component.Props.Get("RECURRENCE-ID") == nil {
			foundBase = true
		}

		// Check for the first instance update
		recurrenceID := component.Props.Get("RECURRENCE-ID")
		if recurrenceID != nil && recurrenceID.Value == "20230108T120000Z" {
			foundInstance1 = true
			summary := component.Props.Get("SUMMARY")
			if summary == nil || summary.Value != "Modified Instance 1" {
				t.Errorf("Instance 1 doesn't have the correct summary")
			}
		}

		// Check for the second instance update
		if recurrenceID != nil && recurrenceID.Value == "20230115T120000Z" {
			foundInstance2 = true
			summary := component.Props.Get("SUMMARY")
			if summary == nil || summary.Value != "Modified Instance 2" {
				t.Errorf("Instance 2 doesn't have the correct summary")
			}
		}
	}

	if eventCount != 3 {
		t.Errorf("Expected 3 VEVENT components, got %d", eventCount)
	}

	if !foundBase {
		t.Errorf("Base recurring event not found in the calendar")
	}

	if !foundInstance1 {
		t.Errorf("First instance update not found in the calendar")
	}

	if !foundInstance2 {
		t.Errorf("Second instance update not found in the calendar")
	}
}

// TestParentUpdateSkipsInstances tests the issue described in Test 17
// where a parent event update with a higher sequence number causes
// instance updates with lower sequence numbers to be incorrectly skipped
func TestParentUpdateSkipsInstances(t *testing.T) {
	// Create processor with in-memory storage
	store := memory.NewMemoryStorage()
	proc := NewProcessor(store, false)

	// Test UID for all events
	uid := "test-recurring-event-issue17"

	// First create parent event with high sequence number (7)
	parentEmail := `From: organizer@example.com
To: attendee@example.com
Subject: Meeting Update (Master)
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="boundary"

--boundary
Content-Type: text/plain

This is the master event update with sequence 7.

--boundary
Content-Type: text/calendar; method=REQUEST; charset=UTF-8
Content-Transfer-Encoding: 7bit

BEGIN:VCALENDAR
PRODID:-//Test//EN
VERSION:2.0
METHOD:REQUEST
BEGIN:VEVENT
SUMMARY:Weekly Meeting (Master Update)
DTSTART:20250505T100000Z
DTEND:20250505T110000Z
UID:test-recurring-event-issue17
SEQUENCE:7
RRULE:FREQ=WEEKLY;COUNT=5
ORGANIZER:organizer@example.com
DTSTAMP:20250501T120000Z
END:VEVENT
END:VCALENDAR

--boundary--
`
	// Process the parent event first
	parentResult, err := proc.ProcessEmail(bytes.NewBufferString(parentEmail))
	if err != nil {
		t.Fatalf("Failed to process parent email: %v", err)
	}
	t.Logf("Parent result: %s", parentResult)

	// Now create and process instance updates with lower sequence numbers
	instances := []struct {
		date     string
		sequence int
		content  string
	}{
		{"20250512", 0, `From: organizer@example.com
To: attendee@example.com
Subject: Instance Update 1
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="boundary"

--boundary
Content-Type: text/plain

This is an instance update with sequence 0.

--boundary
Content-Type: text/calendar; method=REQUEST; charset=UTF-8
Content-Transfer-Encoding: 7bit

BEGIN:VCALENDAR
PRODID:-//Test//EN
VERSION:2.0
METHOD:REQUEST
BEGIN:VEVENT
SUMMARY:Weekly Meeting (Instance 1)
DTSTART:20250512T140000Z
DTEND:20250512T150000Z
UID:test-recurring-event-issue17
SEQUENCE:0
RECURRENCE-ID:20250512T100000Z
ORGANIZER:organizer@example.com
DTSTAMP:20250430T120000Z
END:VEVENT
END:VCALENDAR

--boundary--
`},
		{"20250519", 2, `From: organizer@example.com
To: attendee@example.com
Subject: Instance Update 2
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="boundary"

--boundary
Content-Type: text/plain

This is an instance update with sequence 2.

--boundary
Content-Type: text/calendar; method=REQUEST; charset=UTF-8
Content-Transfer-Encoding: 7bit

BEGIN:VCALENDAR
PRODID:-//Test//EN
VERSION:2.0
METHOD:REQUEST
BEGIN:VEVENT
SUMMARY:Weekly Meeting (Instance 2)
DTSTART:20250519T150000Z
DTEND:20250519T160000Z
UID:test-recurring-event-issue17
SEQUENCE:2
RECURRENCE-ID:20250519T100000Z
ORGANIZER:organizer@example.com
DTSTAMP:20250430T130000Z
END:VEVENT
END:VCALENDAR

--boundary--
`},
		{"20250526", 3, `From: organizer@example.com
To: attendee@example.com
Subject: Instance Update 3
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="boundary"

--boundary
Content-Type: text/plain

This is an instance update with sequence 3.

--boundary
Content-Type: text/calendar; method=REQUEST; charset=UTF-8
Content-Transfer-Encoding: 7bit

BEGIN:VCALENDAR
PRODID:-//Test//EN
VERSION:2.0
METHOD:REQUEST
BEGIN:VEVENT
SUMMARY:Weekly Meeting (Instance 3)
DTSTART:20250526T160000Z
DTEND:20250526T170000Z
UID:test-recurring-event-issue17
SEQUENCE:3
RECURRENCE-ID:20250526T100000Z
ORGANIZER:organizer@example.com
DTSTAMP:20250430T140000Z
END:VEVENT
END:VCALENDAR

--boundary--
`},
	}

	// Process instance updates after the parent update
	skippedCount := 0
	for i, inst := range instances {
		result, err := proc.ProcessEmail(bytes.NewBufferString(inst.content))
		if err != nil {
			t.Fatalf("Failed to process instance %d update: %v", i+1, err)
		}
		t.Logf("Instance %d result: %s", i+1, result)

		// Check if the instance was skipped
		if result == fmt.Sprintf("Not processing older event (sequence: %d vs 7, DTSTAMP comparison) with UID %s", 
			inst.sequence, uid) {
			skippedCount++
		}
	}

	// This test should FAIL because instances should not be skipped
	// just because the parent has a higher sequence number
	if skippedCount > 0 {
		t.Errorf("BUG DETECTED: %d of %d instance updates were incorrectly skipped due to parent's higher sequence number", 
			skippedCount, len(instances))
	} else {
		t.Logf("No bug detected - all instance updates were correctly processed")
	}

	// Verify the final state of the calendar
	event, err := store.GetEvent(uid)
	if err != nil {
		t.Fatalf("Failed to retrieve event: %v", err)
	}

	cal, err := ical.DecodeCalendar(event.RawData)
	if err != nil {
		t.Fatalf("Failed to parse calendar data: %v", err)
	}

	// Count the instance exceptions - there should be one for each instance update
	instanceCount := 0
	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}
		if component.Props.Get("RECURRENCE-ID") != nil {
			instanceCount++
		}
	}

	// The calendar should contain all instance exceptions
	// This test should FAIL because the current implementation drops all instances
	if instanceCount < len(instances) {
		t.Errorf("BUG DETECTED: Calendar contains only %d instance exceptions, but should have %d",
			instanceCount, len(instances))
	} else {
		t.Logf("Correct calendar state: contains %d instance exceptions as expected", instanceCount)
	}
}

// TestInstancesBeforeParent tests the reverse scenario where instances are processed
// before the parent - this helps diagnose how the system should work
func TestInstancesBeforeParent(t *testing.T) {
	// Create processor with in-memory storage
	store := memory.NewMemoryStorage()
	proc := NewProcessor(store, false)

	// Test UID for all events
	uid := "test-recurring-event-reverse"

	// First create and process instance updates with lower sequence numbers
	instances := []struct {
		date     string
		sequence int
		content  string
	}{
		{"20250512", 0, `From: organizer@example.com
To: attendee@example.com
Subject: Instance Update 1
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="boundary"

--boundary
Content-Type: text/plain

This is an instance update with sequence 0.

--boundary
Content-Type: text/calendar; method=REQUEST; charset=UTF-8
Content-Transfer-Encoding: 7bit

BEGIN:VCALENDAR
PRODID:-//Test//EN
VERSION:2.0
METHOD:REQUEST
BEGIN:VEVENT
SUMMARY:Weekly Meeting (Instance 1)
DTSTART:20250512T140000Z
DTEND:20250512T150000Z
UID:test-recurring-event-reverse
SEQUENCE:0
RECURRENCE-ID:20250512T100000Z
ORGANIZER:organizer@example.com
DTSTAMP:20250430T120000Z
END:VEVENT
END:VCALENDAR

--boundary--
`},
		{"20250519", 2, `From: organizer@example.com
To: attendee@example.com
Subject: Instance Update 2
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="boundary"

--boundary
Content-Type: text/plain

This is an instance update with sequence 2.

--boundary
Content-Type: text/calendar; method=REQUEST; charset=UTF-8
Content-Transfer-Encoding: 7bit

BEGIN:VCALENDAR
PRODID:-//Test//EN
VERSION:2.0
METHOD:REQUEST
BEGIN:VEVENT
SUMMARY:Weekly Meeting (Instance 2)
DTSTART:20250519T150000Z
DTEND:20250519T160000Z
UID:test-recurring-event-reverse
SEQUENCE:2
RECURRENCE-ID:20250519T100000Z
ORGANIZER:organizer@example.com
DTSTAMP:20250430T130000Z
END:VEVENT
END:VCALENDAR

--boundary--
`},
	}

	// Process instance updates first
	for i, inst := range instances {
		result, err := proc.ProcessEmail(bytes.NewBufferString(inst.content))
		if err != nil {
			t.Fatalf("Failed to process instance %d update: %v", i+1, err)
		}
		t.Logf("Instance %d result: %s", i+1, result)
	}

	// Verify instances were stored before parent
	beforeEvent, err := store.GetEvent(uid)
	if err != nil {
		t.Logf("Event not found before parent update - this is expected for first instance only")
	} else {
		beforeCal, err := ical.DecodeCalendar(beforeEvent.RawData)
		if err != nil {
			t.Fatalf("Failed to parse calendar data: %v", err)
		}

		beforeInstanceCount := 0
		for _, component := range beforeCal.Children {
			if component.Name == "VEVENT" && component.Props.Get("RECURRENCE-ID") != nil {
				beforeInstanceCount++
			}
		}
		t.Logf("Before parent update: calendar has %d instance exceptions", beforeInstanceCount)
	}

	// Now create and process parent event with high sequence number (7)
	parentEmail := `From: organizer@example.com
To: attendee@example.com
Subject: Meeting Update (Master)
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="boundary"

--boundary
Content-Type: text/plain

This is the master event update with sequence 7.

--boundary
Content-Type: text/calendar; method=REQUEST; charset=UTF-8
Content-Transfer-Encoding: 7bit

BEGIN:VCALENDAR
PRODID:-//Test//EN
VERSION:2.0
METHOD:REQUEST
BEGIN:VEVENT
SUMMARY:Weekly Meeting (Master Update)
DTSTART:20250505T100000Z
DTEND:20250505T110000Z
UID:test-recurring-event-reverse
SEQUENCE:7
RRULE:FREQ=WEEKLY;COUNT=5
ORGANIZER:organizer@example.com
DTSTAMP:20250501T120000Z
END:VEVENT
END:VCALENDAR

--boundary--
`
	
	// Process the parent event after the instance updates
	parentResult, err := proc.ProcessEmail(bytes.NewBufferString(parentEmail))
	if err != nil {
		t.Fatalf("Failed to process parent email: %v", err)
	}
	t.Logf("Parent result: %s", parentResult)

	// Verify the final state
	event, err := store.GetEvent(uid)
	if err != nil {
		t.Fatalf("Failed to retrieve event: %v", err)
	}

	cal, err := ical.DecodeCalendar(event.RawData)
	if err != nil {
		t.Fatalf("Failed to parse calendar data: %v", err)
	}

	// Count the master and instance components
	var masterFound bool
	var finalInstanceCount int

	for _, component := range cal.Children {
		if component.Name != "VEVENT" {
			continue
		}
		
		if component.Props.Get("RECURRENCE-ID") == nil {
			// This is the master event
			masterFound = true
			
			sequence := component.Props.Get("SEQUENCE")
			if sequence != nil {
				t.Logf("Master event sequence: %s", sequence.Value)
			}
		} else {
			// This is an instance
			finalInstanceCount++
			recurrenceID := component.Props.Get("RECURRENCE-ID").Value
			sequence := component.Props.Get("SEQUENCE").Value
			t.Logf("Found instance with RECURRENCE-ID=%s, SEQUENCE=%s", recurrenceID, sequence)
		}
	}

	// Document the current behavior
	if !masterFound {
		t.Errorf("Master event not found after parent update")
	}
	
	// Check if instances were preserved or overwritten
	if finalInstanceCount < len(instances) {
		t.Logf("CURRENT BEHAVIOR: Parent update overwrote instance updates (instances: %d/%d)",
			finalInstanceCount, len(instances))
	} else {
		t.Logf("CURRENT BEHAVIOR: Instance updates were preserved after parent update")
	}
}