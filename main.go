package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mkbrechtel/calmailproc/processor"
	"github.com/mkbrechtel/calmailproc/storage"
	"github.com/mkbrechtel/calmailproc/storage/icalfile"
	"github.com/mkbrechtel/calmailproc/storage/vdir"
)

func main() {
	// Define flags
	jsonOutput := flag.Bool("json", false, "Output in JSON format")
	storeEvent := flag.Bool("store", false, "Store calendar event if found")
	
	// Storage options
	vdirPath := flag.String("vdir", "", "Path to vdir storage directory")
	icalfilePath := flag.String("icalfile", "", "Path to single iCalendar file storage")
	
	flag.Parse()

	// Initialize the appropriate storage
	var (
		store storage.Storage
		err   error
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
		store, err = icalfile.NewICalFileStorage(*icalfilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing icalfile storage: %v\n", err)
			os.Exit(1)
		}
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
	proc := processor.NewProcessor(store)

	// Process the email
	if err := proc.ProcessEmail(os.Stdin, *jsonOutput, *storeEvent); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
