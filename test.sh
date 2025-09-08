#!/bin/bash
#
# Simple test script for calmailproc
# Tests calendar email processing with vdir storage
#
set -e

calmailproc="go run main.go"

# Clean up test directories
rm -rf "test/out"

echo "=== Testing vdir storage ==="

storage_dir="test/out/vdir"
args="-process-replies -vdir $storage_dir"

# Process all example emails
for mail in test/maildir/cur/test-*.eml; do
    # Process the email
    echo
    echo "Processing $mail with vdir storage"

    # Determine expected exit code based on mail file
    expected_exit_code=0
    if [[ "$mail" == *"test-12.eml" ]] || [[ "$mail" == *"test-13.eml" ]] || [[ "$mail" == *"test-14.eml" ]] || [[ "$mail" == *"test-15.eml" ]]; then
        expected_exit_code=1
    fi

    # Run command and capture exit code
    set +e
    $calmailproc $args < "$mail"
    actual_exit_code=$?
    set -e
            
    # Check if exit code matches expected
    if [ $actual_exit_code -eq $expected_exit_code ]; then
        if [ $expected_exit_code -ne 0 ]; then
            echo "✓ Expected error occurred (exit code $actual_exit_code)"
        fi
    else
        echo "✗ ERROR: Expected exit code $expected_exit_code but got $actual_exit_code"
        exit 1
    fi
done

# Simple verification
file_count=$(find "$storage_dir" -type f | wc -l)
echo "vdir: Found $file_count calendar files"