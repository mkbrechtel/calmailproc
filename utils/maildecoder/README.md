# Mail Attachment Decoder

A simple tool to decode and view email attachments, especially useful for examining calendar invitations.

## Features

- Decode and display email file structure and attachments
- View text content of email bodies and attachments
- Extract all attachments to files
- Support for base64 and quoted-printable encodings
- Handles nested multipart emails
- Detects common content types including calendar invitations (.ics files)

## Usage

```
./maildecoder -email=path/to/email.eml [options]
```

### Options

- `-email`: Path to the email file (required)
- `-out`: Output directory for attachments (default: "attachments")
- `-extract-all`: Save all attachments to files
- `-print-text`: Print text attachments to stdout (default: true)

### Examples

```bash
# View an email including any text attachments:
./maildecoder -email=/path/to/email.eml

# Extract all attachments from an email to files:
./maildecoder -email=/path/to/email.eml -extract-all

# Extract attachments to a custom directory:
./maildecoder -email=/path/to/email.eml -extract-all -out=my-attachments

# View only email metadata (no text content):
./maildecoder -email=/path/to/email.eml -print-text=false
```

## Building

```bash
cd utils/maildecoder
go build -o maildecoder
```