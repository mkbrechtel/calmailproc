package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// Parse command line arguments
	emailPath := flag.String("email", "", "Path to email file")
	outputDir := flag.String("out", "attachments", "Output directory for attachments")
	extractAll := flag.Bool("extract-all", false, "Extract all attachments to files")
	printText := flag.Bool("print-text", true, "Print text attachments to stdout")
	flag.Parse()

	// Check if email path is provided
	if *emailPath == "" {
		fmt.Fprintln(os.Stderr, "Error: email path is required")
		fmt.Fprintln(os.Stderr, "Usage: maildecoder -email=path/to/email.eml [-out=output_dir] [-extract-all] [-print-text=false]")
		os.Exit(1)
	}

	// Create config and run decoder
	config := &AttachConfig{
		EmailPath:  *emailPath,
		OutputDir:  *outputDir,
		ExtractAll: *extractAll,
		PrintText:  *printText,
	}
	
	if err := RunAttach(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}