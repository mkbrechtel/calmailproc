#!/bin/bash
#
# Simple test script for calmailproc
# Tests calendar email processing without modifying the filesystem

set -e

calmailproc="go run main.go"

echo "=== Testing maildir mode ==="
# Use existing test maildir without storing events (just parse and print)
$calmailproc -maildir test/maildir -verbose

echo
echo "=== Testing single email processing mode ==="
# Test processing a single email file without storage
echo "Processing example-mail-01.eml"
$calmailproc < test/maildir/cur/example-mail-01.eml
