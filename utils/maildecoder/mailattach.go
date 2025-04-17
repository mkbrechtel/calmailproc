package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
)

// AttachConfig contains config for attachment extraction
type AttachConfig struct {
	EmailPath   string
	OutputDir   string
	ExtractAll  bool
	PrintText   bool
}

// RunAttach extracts and processes email attachments
func RunAttach(config *AttachConfig) error {
	if config.EmailPath == "" {
		return fmt.Errorf("email path is required")
	}

	// Open the email file
	file, err := os.Open(config.EmailPath)
	if err != nil {
		return fmt.Errorf("opening email file: %w", err)
	}
	defer file.Close()

	// Parse the email
	msg, err := mail.ReadMessage(file)
	if err != nil {
		return fmt.Errorf("parsing email: %w", err)
	}

	fmt.Printf("Subject: %s\n", msg.Header.Get("Subject"))
	fmt.Printf("From: %s\n", msg.Header.Get("From"))
	fmt.Printf("To: %s\n", msg.Header.Get("To"))
	fmt.Printf("Date: %s\n\n", msg.Header.Get("Date"))

	// Create output directory if extracting all attachments
	if config.ExtractAll {
		if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
	}

	// Get content type and process accordingly
	contentType := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// If Content-Type parsing fails, treat body as text
		body, err := io.ReadAll(msg.Body)
		if err != nil {
			return fmt.Errorf("reading email body: %w", err)
		}
		if config.PrintText {
			fmt.Println("--- MESSAGE BODY ---")
			fmt.Println(string(body))
		}
		return nil
	}

	// Process multipart email
	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return fmt.Errorf("no boundary found for multipart message")
		}
		
		err = processAttachmentsMultipart(msg.Body, boundary, config)
		if err != nil {
			return fmt.Errorf("processing attachments: %w", err)
		}
	} else {
		// Handle single part email
		body, err := io.ReadAll(msg.Body)
		if err != nil {
			return fmt.Errorf("reading email body: %w", err)
		}
		
		if config.PrintText && isTextType(mediaType) {
			fmt.Println("--- MESSAGE BODY ---")
			fmt.Println(string(body))
		}
	}

	return nil
}

// processAttachmentsMultipart processes each part of a multipart message
func processAttachmentsMultipart(r io.Reader, boundary string, config *AttachConfig) error {
	mr := multipart.NewReader(r, boundary)
	partNum := 0

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading next part: %w", err)
		}
		partNum++

		// Get part's content type
		partContentType := part.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(partContentType)
		if err != nil {
			// Skip parts with invalid content type
			continue
		}

		// Get filename from Content-Disposition if available
		fileName := ""
		contentDisposition := part.Header.Get("Content-Disposition")
		if contentDisposition != "" {
			_, dispParams, err := mime.ParseMediaType(contentDisposition)
			if err == nil && dispParams["filename"] != "" {
				fileName = dispParams["filename"]
			}
		}

		// Process part based on content type
		if strings.HasPrefix(mediaType, "multipart/") {
			// Handle nested multipart
			nestedBoundary := params["boundary"]
			if nestedBoundary != "" {
				partData, err := io.ReadAll(part)
				if err != nil {
					return fmt.Errorf("reading nested multipart data: %w", err)
				}
				err = processAttachmentsMultipart(strings.NewReader(string(partData)), nestedBoundary, config)
				if err != nil {
					return fmt.Errorf("processing nested multipart: %w", err)
				}
			}
		} else {
			// Process regular part
			partData, err := io.ReadAll(part)
			if err != nil {
				return fmt.Errorf("reading part data: %w", err)
			}

			// Decode the content if needed
			contentTransferEncoding := part.Header.Get("Content-Transfer-Encoding")
			decoded, err := decodeContent(partData, contentTransferEncoding)
			if err != nil {
				return fmt.Errorf("decoding content: %w", err)
			}

			// If no filename, generate one based on part number and content type
			if fileName == "" {
				ext := getExtensionFromContentType(mediaType)
				fileName = fmt.Sprintf("part_%d%s", partNum, ext)
			}

			// Print text content to stdout
			if config.PrintText && isTextType(mediaType) {
				fmt.Printf("\n--- ATTACHMENT: %s (%s) ---\n", fileName, mediaType)
				fmt.Println(string(decoded))
			}

			// Save attachment to file if extracting all
			if config.ExtractAll {
				outputPath := filepath.Join(config.OutputDir, fileName)
				err = os.WriteFile(outputPath, decoded, 0644)
				if err != nil {
					return fmt.Errorf("saving attachment: %w", err)
				}
				fmt.Printf("Saved attachment: %s\n", outputPath)
			}
		}
	}

	return nil
}

// decodeContent decodes content based on Content-Transfer-Encoding
func decodeContent(data []byte, encoding string) ([]byte, error) {
	encoding = strings.ToLower(encoding)
	
	switch encoding {
	case "base64":
		decoded := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
		n, err := base64.StdEncoding.Decode(decoded, data)
		if err != nil {
			return nil, fmt.Errorf("base64 decoding: %w", err)
		}
		return decoded[:n], nil
	case "quoted-printable":
		// Basic quoted-printable handling
		// For a more complete implementation, consider using a dedicated library
		result := []byte{}
		i := 0
		for i < len(data) {
			if data[i] == '=' && i+2 < len(data) {
				// Try to decode hex value
				hex := string(data[i+1:i+3])
				if val, err := decodeHex(hex); err == nil {
					result = append(result, val)
					i += 3
					continue
				}
			}
			result = append(result, data[i])
			i++
		}
		return result, nil
	default:
		// For 7bit, 8bit, binary or unspecified, return as is
		return data, nil
	}
}

// decodeHex decodes a hex string to byte
func decodeHex(s string) (byte, error) {
	var val byte
	_, err := fmt.Sscanf(s, "%02x", &val)
	return val, err
}

// isTextType checks if the content type is text-based
func isTextType(contentType string) bool {
	return strings.HasPrefix(contentType, "text/") ||
		contentType == "application/json" ||
		contentType == "application/xml" ||
		contentType == "application/javascript"
}

// getExtensionFromContentType returns a file extension based on content type
func getExtensionFromContentType(contentType string) string {
	switch {
	case strings.Contains(contentType, "text/plain"):
		return ".txt"
	case strings.Contains(contentType, "text/html"):
		return ".html"
	case strings.Contains(contentType, "application/pdf"):
		return ".pdf"
	case strings.Contains(contentType, "image/jpeg"):
		return ".jpg"
	case strings.Contains(contentType, "image/png"):
		return ".png"
	case strings.Contains(contentType, "text/calendar"):
		return ".ics"
	default:
		return ""
	}
}