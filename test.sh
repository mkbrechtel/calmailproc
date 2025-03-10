#!/bin/bash
#
# Simple test script for calmailproc
# Tests calendar email processing with different storage methods

set -e

calmailproc="go run main.go"
test_dir="test"
mail_dir="$test_dir/mails"

# Clean up test directories
rm -rf "$test_dir/vdir"
rm -f "$test_dir/calendar.ics"

# Loop through storage methods
for method in "vdir" "icalfile"; do
    echo "=== Testing $method storage ==="
    
    # Set up args based on method
    if [ "$method" == "vdir" ]; then
        storage_dir="$test_dir/vdir"
        args="-store -vdir $storage_dir"
    else
        storage_file="$test_dir/calendar.ics"
        args="-store -icalfile $storage_file"
    fi
    
    # Process all example emails
    for mail in $mail_dir/example-mail-*.eml; do
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
