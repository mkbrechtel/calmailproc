# calmailproc

A calendar mail processor that extracts calendar event information from email files (.eml) containing iCalendar (.ics) data.

## Features

- Reads email messages from stdin
- Parses email headers and extracts relevant information
- Identifies and extracts calendar event data from the email
- Outputs processed data in plain text or JSON format
- Designed to work in Unix mail processing pipelines

## Installation

```bash
# Clone the repository
git clone https://github.com/mkbrechtel/calmailproc.git
cd calmailproc

# Build the application
go build -o calmailproc ./cmd/calmailproc
```

## Usage

```bash
# Process an email file and display information in plain text
cat email.eml | calmailproc

# Process an email file and output JSON format
cat email.eml | calmailproc -json
```

### Integration with mail systems

The tool is designed to be used in standard Unix mail pipelines. For example:

```bash
# Process incoming mail with procmail
:0c
* ^Content-Type:.*text/calendar
| calmailproc > /path/to/calendar/event.txt
```

### Example output

Plain text:
```
Subject: Test Event 1
From: Markus Brechtel <markus.brechtel@uk-koeln.de>
To: "user@example.com" <user@example.com>
Date: 2025-03-10 09:41:35

Calendar Event:
  Summary: Test Event 1
  Start: 2025-03-10 14:00:00
  End: 2025-03-10 15:00:00
  Location: Meeting Room A
  Organizer: Markus Brechtel
```

JSON:
```json
{
  "subject": "Test Event 1",
  "from": "Markus Brechtel <markus.brechtel@uk-koeln.de>",
  "to": "\"user@example.com\" <user@example.com>",
  "date": "2025-03-10T09:41:35Z",
  "has_calendar": true,
  "event": {
    "summary": "Test Event 1",
    "start": "2025-03-10T14:00:00+01:00",
    "end": "2025-03-10T15:00:00+01:00",
    "location": "Meeting Room A",
    "organizer": "Markus Brechtel"
  }
}
```

## Future Improvements

- Improved iCalendar parsing with proper timezone handling
- Support for recurring events
- Support for meeting acceptances/declines
- Additional output formats (e.g., vCard)
- Calendar event filtering options

## License

Apache License 2.0
