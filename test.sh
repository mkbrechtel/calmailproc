#!/bin/bash
#
# Simple test script for calmailproc
# Tests calendar email processing with different storage methods

set -e

calmailproc="go run main.go"

# Clean up test directories
rm -rf "test/out"


echo "=== Testing maildir mode ==="
$calmailproc -process-replies -maildir test/maildir -vdir test/out/vdir1

# Loop through storage methods
for method in "vdir" "icalfile"; do
    echo "=== Testing $method storage ==="
    
    # Set up args based on method
    if [ "$method" == "vdir" ]; then
        storage_dir="test/out/vdir2"
        args="-process-replies -vdir $storage_dir"
    else
        storage_file="test/out/calendar.ics"
        args="-process-replies -icalfile $storage_file"
    fi
    
    # Process all example emails
    for mail in test/maildir/cur/example-mail-*.eml; do
       # Process the email
        echo
        echo "Processing $mail with $method storage"
        $calmailproc $args < "$mail"
    done
    
    # Simple verification
    if [ "$method" == "vdir" ]; then
        file_count=$(find "$storage_dir" -type f | wc -l)
        echo "$method: Found $file_count calendar files"
    else
        if [ -f "$storage_file" ]; then
            event_count=$(grep -c "BEGIN:VEVENT" "$storage_file")
            echo "$method: Found $event_count events in calendar file"
        else
            echo "ERROR: Calendar file not created"
            exit 1
        fi
    fi
done
