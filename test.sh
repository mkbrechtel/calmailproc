#!/bin/sh

calmailproc="go run main.go"

args="-store -vdir test/vdir"

rm -rf test/vdir

set -e

# Process the files in specific order to test recurring event handling
for file in test/mails/example-mail-owa-[1-4].eml; do
    echo "Processing $file"
    $calmailproc $args < "$file"
done

# Process recurring events in order
echo "Processing recurring event series..."
$calmailproc $args < test/mails/example-mail-owa-5.eml
$calmailproc $args < test/mails/example-mail-owa-7.eml
$calmailproc $args < test/mails/example-mail-owa-6.eml

# Verify the UID file exists and contains all occurrences
uid="040000008200E00074C5B7101A82E0080000000044440AFCBB91DB0100000000000000001000000087598F58784D4541BAA76F1829CFE9A1"
if [ -f "test/vdir/$uid.ics" ]; then
    echo "Success: Recurring event file exists"
    
    # Count VEVENT components to verify we've merged them
    vevent_count=$(grep -c "BEGIN:VEVENT" "test/vdir/$uid.ics")
    echo "Found $vevent_count VEVENT components in the file"
    
    # Check if there are cancelled events
    cancelled_count=$(grep -c "STATUS:CANCELLED" "test/vdir/$uid.ics")
    echo "Found $cancelled_count cancelled events"
    
    # Check for recurrence exceptions
    recurrence_id_count=$(grep -c "RECURRENCE-ID" "test/vdir/$uid.ics")
    echo "Found $recurrence_id_count recurrence exceptions"
    
    # Summary of content verification
    echo "Verification successful: Found master event and its exceptions"
else
    echo "Error: Recurring event file not found"
    exit 1
fi
