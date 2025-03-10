package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mkbrechtel/calmailproc/processor"
	"github.com/mkbrechtel/calmailproc/storage/vdir"
)

func main() {
	// Define flags
	jsonOutput := flag.Bool("json", false, "Output in JSON format")
	storeEvent := flag.Bool("store", false, "Store calendar event if found")
	vdirPath := flag.String("vdir", filepath.Join(os.Getenv("HOME"), ".calendar"), "Path to vdir storage directory")
	flag.Parse()

	// Initialize the storage
	storage, err := vdir.NewVDirStorage(*vdirPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing storage: %v\n", err)
		os.Exit(1)
	}

	// Initialize the processor
	proc := processor.NewProcessor(storage)

	// Process the email
	if err := proc.ProcessEmail(os.Stdin, *jsonOutput, *storeEvent); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
