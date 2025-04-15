package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mkbrechtel/calmailproc/processor"
	"github.com/mkbrechtel/calmailproc/processor/maildir"
	"github.com/mkbrechtel/calmailproc/processor/stdin"
	"github.com/mkbrechtel/calmailproc/storage"
	"github.com/mkbrechtel/calmailproc/storage/icalfile"
	"github.com/mkbrechtel/calmailproc/storage/vdir"
)

func main() {
	// Define flags
	jsonOutput := flag.Bool("json", false, "Output in JSON format")
	storeEvent := flag.Bool("store", false, "Store calendar event if found")
	processReplies := flag.Bool("process-replies", true, "Process attendance replies to update events")

	// Storage options
	vdirPath := flag.String("vdir", "", "Path to vdir storage directory")
	icalfilePath := flag.String("icalfile", "", "Path to single iCalendar file storage")

	// Input options
	maildirPath := flag.String("maildir", "", "Path to maildir to process (will process all emails recursively)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging output")

	flag.Parse()

	// Initialize the appropriate storage
	var (
		store storage.Storage
		err   error
		icalStorage *icalfile.ICalFileStorage
	)

	switch {
	case *vdirPath != "":
		// Use vdir storage if specified
		store, err = vdir.NewVDirStorage(*vdirPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing vdir storage: %v\n", err)
			os.Exit(1)
		}
	case *icalfilePath != "":
		// Use icalfile storage if specified
		icalStorage, err = icalfile.NewICalFileStorage(*icalfilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing icalfile storage: %v\n", err)
			os.Exit(1)
		}
		// Open the storage for operations
		if err := icalStorage.OpenAndLock(); err != nil {
			fmt.Fprintf(os.Stderr, "Error opening icalfile storage: %v\n", err)
			os.Exit(1)
		}
		store = icalStorage
		// Make sure to close and write the storage before exiting
		defer func() {
			if err := icalStorage.WriteAndUnlock(); err != nil {
				fmt.Fprintf(os.Stderr, "Error closing icalfile storage: %v\n", err)
			}
		}()
	default:
		// Default to vdir in user's home directory
		defaultPath := filepath.Join(os.Getenv("HOME"), ".calendar")
		store, err = vdir.NewVDirStorage(defaultPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing default storage: %v\n", err)
			os.Exit(1)
		}
	}

	// Initialize the processor
	proc := processor.NewProcessor(store, *processReplies)

	// Process maildir if specified
	if *maildirPath != "" {
		if err := maildir.Process(*maildirPath, proc, *jsonOutput, *storeEvent, *verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing maildir: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Default: process from stdin
	if err := stdin.Process(proc, *jsonOutput, *storeEvent); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}