#!/bin/bash
#
# Simple test script for calmailproc
# Tests calendar email processing with vdir and CalDAV storage
#
set -e

calmailproc="go run main.go"
RADICALE_PID=""

# Cleanup function to ensure Radicale is stopped
cleanup() {
    if [ -n "$RADICALE_PID" ]; then
        echo "Stopping Radicale server (PID: $RADICALE_PID)..."
        kill $RADICALE_PID 2>/dev/null || true
        wait $RADICALE_PID 2>/dev/null || true
    fi
}

# Set up trap to cleanup on exit
trap cleanup EXIT

# Clean up test directories
rm -rf "test/out"
mkdir -p "test/out/caldav"

echo "=== Testing maildir mode ==="
$calmailproc -process-replies -maildir test/maildir -vdir test/out/vdir-from-maildir -verbose

echo
echo "=== Testing vdir storage (single file imports) ==="
storage_dir="test/out/vdir-single-imports"
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

# Verify vdir storage
file_count=$(find "$storage_dir" -type f | wc -l)
echo "vdir (single imports): Found $file_count calendar files"

# Compare maildir and single-import results
maildir_count=$(find "test/out/vdir-from-maildir" -type f | wc -l)
echo "vdir (from maildir): Found $maildir_count calendar files"

if [ "$file_count" != "$maildir_count" ]; then
    echo "✗ ERROR: File count mismatch between maildir ($maildir_count) and single imports ($file_count)"
    exit 1
fi

echo
echo "=== Testing CalDAV storage ==="

# Start Radicale server
echo "Starting Radicale CalDAV server..."
radicale --storage-filesystem-folder=./test/out/caldav \
         --auth-type=none \
         --hosts='localhost:15232' &
RADICALE_PID=$!

# Wait for Radicale to start
echo "Waiting for Radicale to start..."
for i in {1..10}; do
    if curl -s -f http://localhost:15232/ >/dev/null 2>&1; then
        echo "Radicale is ready"
        break
    fi
    if [ $i -eq 10 ]; then
        echo "ERROR: Radicale failed to start"
        exit 1
    fi
    sleep 0.5
done

# Create test calendar
echo "Creating test calendar..."
curl -X MKCALENDAR http://test:pass@localhost:15232/test/calendar/ >/dev/null 2>&1 || true

caldav_url="http://test:pass@localhost:15232/test/calendar/"
args="-process-replies -caldav $caldav_url"

# Process all example emails with CalDAV
for mail in test/maildir/cur/test-*.eml; do
    # Skip test 16 in CalDAV mode
    if [[ "$mail" == *"test-16-"*.eml ]]; then
        echo
        echo "Skipping $mail in CalDAV mode"
        continue
    fi
    
    # Process the email
    echo
    echo "Processing $mail with CalDAV storage"

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

# Verify CalDAV storage
echo
echo "Verifying CalDAV storage..."
event_count=$(curl -s -X PROPFIND http://test:pass@localhost:15232/test/calendar/ \
    -H "Depth: 1" \
    -H "Content-Type: application/xml" \
    -d '<?xml version="1.0" encoding="utf-8"?><propfind xmlns="DAV:"><prop><resourcetype/></prop></propfind>' \
    | grep -o '<response>' | wc -l)
# Subtract 1 for the collection itself
event_count=$((event_count - 1))
echo "CalDAV: Found $event_count calendar events"

echo
echo "✅ All tests passed!"