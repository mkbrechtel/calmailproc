package cli

import (
	"flag"
	"fmt"

	"github.com/mkbrechtel/calmailproc/processor"
	"github.com/mkbrechtel/calmailproc/processor/stdin"
	"github.com/mkbrechtel/calmailproc/storage"
	"github.com/mkbrechtel/calmailproc/storage/caldav"
	"github.com/mkbrechtel/calmailproc/storage/vdir"
)

// Config contains all the CLI configuration options
type Config struct {
	ProcessReplies bool
	VdirPath      string
	CalDAV        string
	Verbose       bool
}

// ParseFlags parses command line flags and returns a Config
func ParseFlags() *Config {
	config := &Config{}

	// Define flags
	flag.BoolVar(&config.ProcessReplies, "process-replies", false, "Process attendance replies to update events")

	// Storage options
	flag.StringVar(&config.VdirPath, "vdir", "", "Path to vdir storage directory")
	flag.StringVar(&config.CalDAV, "caldav", "", "CalDAV URL (e.g., http://user:pass@localhost:5232/calendar/)")
	
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging output")

	flag.Parse()
	return config
}

// Run executes the application with the given configuration
func Run(config *Config) error {
	// Initialize the appropriate storage
	var (
		store storage.Storage
		err   error
	)

	if config.CalDAV != "" {
		// Use CalDAV storage if specified
		store, err = caldav.NewCalDAVStorageFromURL(config.CalDAV)
		if err != nil {
			return fmt.Errorf("error initializing CalDAV storage: %w", err)
		}
	} else if config.VdirPath != "" {
		// Use vdir storage if specified
		store, err = vdir.NewVDirStorage(config.VdirPath)
		if err != nil {
			return fmt.Errorf("error initializing vdir storage: %w", err)
		}
	} else {
		// Default to using vdir storage with "vdir" in current directory
		defaultPath := "vdir"
		store, err = vdir.NewVDirStorage(defaultPath)
		if err != nil {
			return fmt.Errorf("error initializing default vdir storage: %w", err)
		}
	}

	// Initialize the processor
	proc := processor.NewProcessor(store, config.ProcessReplies)

	// Process from stdin
	if err := stdin.Process(proc); err != nil {
		return fmt.Errorf("error processing stdin: %w", err)
	}
	
	return nil
}