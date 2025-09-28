#!/bin/bash
#
# Simple test script for calmailproc
# Tests calendar email processing with CalDAV storage
#
set -e

calmailproc="go run main.go"
XANDIKOS_PID=""

cleanup() {
    if [ -n "$XANDIKOS_PID" ]; then
        echo "Stopping Xandikos server (PID: $XANDIKOS_PID)..."
        kill $XANDIKOS_PID 2>/dev/null || true
        wait $XANDIKOS_PID 2>/dev/null || true
    fi
}

trap cleanup EXIT

rm -rf "test/out"
mkdir -p "test/out/xandikos"

echo "=== Starting Xandikos CalDAV server ==="
xandikos -d ./test/out/xandikos --autocreate -l localhost -p 15232 --no-detect-systemd &
XANDIKOS_PID=$!

echo "Waiting for Xandikos to start..."
for i in {1..10}; do
    if curl -s -f http://localhost:15232/ >/dev/null 2>&1; then
        echo "Xandikos is ready"
        break
    fi
    if [ $i -eq 10 ]; then
        echo "ERROR: Xandikos failed to start"
        exit 1
    fi
    sleep 0.5
done

echo "Creating test calendars..."
curl -X MKCOL http://localhost:15232/user/ >/dev/null 2>&1 || true
curl -X MKCALENDAR http://localhost:15232/user/calendar1/ >/dev/null 2>&1 || true
curl -X MKCALENDAR http://localhost:15232/user/calendar2/ >/dev/null 2>&1 || true
curl -X MKCALENDAR http://localhost:15232/user/calendar3/ >/dev/null 2>&1 || true

echo
echo "=== Testing CalDAV storage (calendar1) ==="

caldav_args1="-process-replies -url http://localhost:15232 -user test -pass pass -calendar /user/calendar1/"

for mail in test/maildir/cur/test-*.eml; do
    echo
    echo "Processing $mail with CalDAV storage (calendar1)"

    expected_exit_code=0
    if [[ "$mail" == *"test-12.eml" ]] || [[ "$mail" == *"test-13.eml" ]] || [[ "$mail" == *"test-14.eml" ]] || [[ "$mail" == *"test-15.eml" ]]; then
        expected_exit_code=1
    fi

    set +e
    $calmailproc $caldav_args1 < "$mail"
    actual_exit_code=$?
    set -e

    if [ $actual_exit_code -eq $expected_exit_code ]; then
        if [ $expected_exit_code -ne 0 ]; then
            echo "✓ Expected error occurred (exit code $actual_exit_code)"
        fi
    else
        echo "✗ ERROR: Expected exit code $expected_exit_code but got $actual_exit_code"
        exit 1
    fi
done

echo
echo "Verifying CalDAV storage (calendar1)..."
event_count1=$(curl -s -X PROPFIND http://localhost:15232/user/calendar1/ \
    -H "Depth: 1" \
    -H "Content-Type: application/xml" \
    -d '<?xml version="1.0" encoding="utf-8"?><propfind xmlns="DAV:"><prop><resourcetype/></prop></propfind>' \
    | grep -o 'href>[^<]*\.ics' | wc -l)
echo "CalDAV calendar1: Found $event_count1 calendar events"

echo
echo "=== Testing maildir mode (calendar2) ==="

caldav_args2="-process-replies -url http://localhost:15232 -user test -pass pass -calendar /user/calendar2/"
$calmailproc -process-replies -maildir test/maildir $caldav_args2 -verbose

echo
echo "Verifying CalDAV storage (calendar2)..."
event_count2=$(curl -s -X PROPFIND http://localhost:15232/user/calendar2/ \
    -H "Depth: 1" \
    -H "Content-Type: application/xml" \
    -d '<?xml version="1.0" encoding="utf-8"?><propfind xmlns="DAV:"><prop><resourcetype/></prop></propfind>' \
    | grep -o 'href>[^<]*\.ics' | wc -l)
echo "CalDAV calendar2: Found $event_count2 calendar events"

echo
echo "=== Comparing results ==="
echo "Calendar1 (stdin mode): $event_count1 events"
echo "Calendar2 (maildir mode): $event_count2 events"

if [ "$event_count1" = "$event_count2" ]; then
    echo "✓ Both calendars have same event count!"
    echo "✓ Maildir and stdin modes produce same results!"
else
    echo "✗ Event count mismatch: calendar1=$event_count1, calendar2=$event_count2"
    exit 1
fi

echo
echo "=== Testing config file mode (calendar3) ==="

config_file="test/config/calmailproc/config.yaml"
temp_config="/tmp/calmailproc-test-config.yaml"
cat > "$temp_config" << EOF
webdav:
  url: http://localhost:15232
  user: test
  pass: pass
  calendar: /user/calendar3/

processor:
  process_replies: true

maildir:
  path: test/maildir
  verbose: true
EOF

echo "Using test config (temporary override):"
cat "$temp_config"

echo
echo "Processing maildir with config file..."
mkdir -p test/config/calmailproc
cp "$temp_config" test/config/calmailproc/config.yaml
XDG_CONFIG_HOME="$(pwd)/test/config" $calmailproc
rm "$temp_config"

echo
echo "Verifying CalDAV storage (calendar3)..."
event_count3=$(curl -s -X PROPFIND http://localhost:15232/user/calendar3/ \
    -H "Depth: 1" \
    -H "Content-Type: application/xml" \
    -d '<?xml version="1.0" encoding="utf-8"?><propfind xmlns="DAV:"><prop><resourcetype/></prop></propfind>' \
    | grep -o 'href>[^<]*\.ics' | wc -l)
echo "CalDAV calendar3: Found $event_count3 calendar events"

echo
echo "=== Comparing all results ==="
echo "Calendar1 (stdin mode):    $event_count1 events"
echo "Calendar2 (maildir mode):  $event_count2 events"
echo "Calendar3 (config file):   $event_count3 events"

if [ "$event_count1" = "$event_count2" ] && [ "$event_count2" = "$event_count3" ]; then
    echo "✓ All three calendars have same event count!"
    echo "✓ All modes produce same results!"
else
    echo "✗ Event count mismatch: calendar1=$event_count1, calendar2=$event_count2, calendar3=$event_count3"
    exit 1
fi

echo
echo "✅ All tests passed!"