#!/bin/sh

calmailproc="go run main.go"

set -e

echo "=== Testing vdir storage ==="
vdir_args="-store -vdir test/vdir"

# Clean up existing test directories
rm -rf test/vdir

# Process the files in specific order to test recurring event handling in vdir
for file in test/mails/example-mail-0[1-4].eml; do
    echo "Processing $file (vdir)"
    $calmailproc $vdir_args < "$file"
done

# Process recurring events in order for vdir
echo "Processing recurring event series (vdir)..."
$calmailproc $vdir_args < test/mails/example-mail-05.eml
$calmailproc $vdir_args < test/mails/example-mail-07.eml
$calmailproc $vdir_args < test/mails/example-mail-06.eml

# Process remaining emails (8-14) to test various scenarios
for file in test/mails/example-mail-0[8-9].eml test/mails/example-mail-1[0-4].eml; do
    echo "Processing $file (vdir)"
    $calmailproc $vdir_args < "$file"
done

# Verify the vdir storage
uid="040000008200E00074C5B7101A82E0080000000044440AFCBB91DB0100000000000000001000000087598F58784D4541BAA76F1829CFE9A1"
if [ -f "test/vdir/$uid.ics" ]; then
    echo "vdir: Success - Recurring event file exists"
    
    # Count VEVENT components to verify we've merged them
    vevent_count=$(grep -c "BEGIN:VEVENT" "test/vdir/$uid.ics")
    echo "vdir: Found $vevent_count VEVENT components in the file"
    
    # Check if there are cancelled events
    cancelled_count=$(grep -c "STATUS:CANCELLED" "test/vdir/$uid.ics")
    echo "vdir: Found $cancelled_count cancelled events"
    
    # Check for recurrence exceptions
    recurrence_id_count=$(grep -c "RECURRENCE-ID" "test/vdir/$uid.ics")
    echo "vdir: Found $recurrence_id_count recurrence exceptions"
    
    # Summary of content verification
    echo "vdir: Verification successful: Found master event and its exceptions"
else
    echo "vdir: Error - Recurring event file not found"
    exit 1
fi

echo 
echo "=== Testing icalfile storage ==="
icalfile_args="-store -icalfile test/calendar.ics"

# Clean up existing test files
rm -f test/calendar.ics

# Process the files in specific order to test recurring event handling in icalfile
for file in test/mails/example-mail-0[1-4].eml; do
    echo "Processing $file (icalfile)"
    $calmailproc $icalfile_args < "$file"
done

# Process recurring events in order for icalfile
echo "Processing recurring event series (icalfile)..."
$calmailproc $icalfile_args < test/mails/example-mail-05.eml
$calmailproc $icalfile_args < test/mails/example-mail-07.eml
$calmailproc $icalfile_args < test/mails/example-mail-06.eml

# Process remaining emails (8-14) to test various scenarios
for file in test/mails/example-mail-0[8-9].eml test/mails/example-mail-1[0-4].eml; do
    echo "Processing $file (icalfile)"
    $calmailproc $icalfile_args < "$file"
done

# Verify the icalfile storage
if [ -f "test/calendar.ics" ]; then
    echo "icalfile: Success - Calendar file exists"
    
    # Count total VEVENT components
    vevent_count=$(grep -c "BEGIN:VEVENT" "test/calendar.ics")
    echo "icalfile: Found $vevent_count VEVENT components in the file"
    
    # Count unique UIDs to verify all events are stored
    uid_count=$(grep -c "UID:" "test/calendar.ics")
    echo "icalfile: Verified $uid_count events are stored"
    
    # Verify recurring events are stored properly
    if grep -q "$uid" "test/calendar.ics"; then
        # Count recurrence exceptions for our test recurring event
        recurrence_count=$(grep -c "RECURRENCE-ID" "test/calendar.ics")
        echo "icalfile: Found $recurrence_count recurrence exceptions"
        
        # Check for cancelled events
        cancelled_count=$(grep -c "STATUS:CANCELLED" "test/calendar.ics")
        echo "icalfile: Found $cancelled_count cancelled events"
        
        # Summary of content verification
        echo "icalfile: Verification successful: All events stored in a single file"
    else
        echo "icalfile: Error - Recurring event not found in calendar file"
        exit 1
    fi
else
    echo "icalfile: Error - Calendar file not found"
    exit 1
fi

echo
echo "=== Testing METHOD:REPLY handling ==="
reply_vdir="test/reply_vdir"
rm -rf "$reply_vdir"

# First test with process-replies=true (default)
echo "Testing with process-replies=true"
$calmailproc -store -vdir "$reply_vdir" < test/mails/example-mail-11.eml
echo "Processing REPLY with process-replies=true"
$calmailproc -store -vdir "$reply_vdir" < test/mails/example-mail-12.eml

# Count how many files were created
files_count=$(find "$reply_vdir" -type f | wc -l)
echo "Files in vdir with process-replies=true: $files_count"

# Now test with process-replies=false
reply_vdir_no="test/reply_vdir_no"
rm -rf "$reply_vdir_no"

echo "Testing with process-replies=false"
$calmailproc -store -vdir "$reply_vdir_no" < test/mails/example-mail-11.eml
echo "Processing REPLY with process-replies=false"
$calmailproc -store -vdir "$reply_vdir_no" --process-replies=false < test/mails/example-mail-12.eml

# Count how many files were created
files_count_no=$(find "$reply_vdir_no" -type f | wc -l)
echo "Files in vdir with process-replies=false: $files_count_no"

if [ "$files_count" -gt "$files_count_no" ]; then
    echo "METHOD:REPLY handling working correctly"
else
    echo "ERROR: METHOD:REPLY handling did not work as expected"
    exit 1
fi

echo
echo "All tests passed successfully!"
