package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/mkbrechtel/calmailproc/processor"
	"github.com/mkbrechtel/calmailproc/processor/maildir"
	"github.com/mkbrechtel/calmailproc/processor/stdin"
	"github.com/mkbrechtel/calmailproc/storage"
	"github.com/mkbrechtel/calmailproc/storage/icalfile"
	"github.com/mkbrechtel/calmailproc/storage/vdir"
)

// Config contains all the CLI configuration options
type Config struct {
	ProcessReplies bool
	VdirPath      string
	IcalfilePath  string
	MaildirPath   string
	Verbose       bool
}

// ParseFlags parses command line flags and returns a Config
func ParseFlags() *Config {
	config := &Config{}

	// Define flags
	flag.BoolVar(&config.ProcessReplies, "process-replies", false, "Process attendance replies to update events")

	// Storage options
	flag.StringVar(&config.VdirPath, "vdir", "", "Path to vdir storage directory (overrides default icalfile storage)")
	flag.StringVar(&config.IcalfilePath, "icalfile", "", "Path to single iCalendar file storage (default: invitations.ics in current directory)")

	// Input options
	flag.StringVar(&config.MaildirPath, "maildir", "", "Path to maildir to process (will process all emails recursively)")
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
		icalStorage *icalfile.ICalFileStorage
	)

	switch {
	case config.VdirPath != "":
		// Use vdir storage if specified
		store, err = vdir.NewVDirStorage(config.VdirPath)
		if err != nil {
			return fmt.Errorf("error initializing vdir storage: %w", err)
		}
	case config.IcalfilePath != "":
		// Use specified icalfile storage
		icalStorage, err = icalfile.NewICalFileStorage(config.IcalfilePath)
		if err != nil {
			return fmt.Errorf("error initializing icalfile storage: %w", err)
		}
		// Open the storage for operations
		if err := icalStorage.ReadAndLockOpen(); err != nil {
			return fmt.Errorf("error opening icalfile storage: %w", err)
		}
		store = icalStorage
		// Make sure to close and write the storage before exiting
		defer func() {
			if err := icalStorage.WriteAndUnlock(); err != nil {
				fmt.Fprintf(os.Stderr, "Error closing icalfile storage: %v\n", err)
			}
		}()
	default:
		// Default to using icalfile storage with invitations.ics in current directory
		defaultPath := "invitations.ics"
		icalStorage, err = icalfile.NewICalFileStorage(defaultPath)
		if err != nil {
			return fmt.Errorf("error initializing default icalfile storage: %w", err)
		}
		// Open the storage for operations
		if err := icalStorage.ReadAndLockOpen(); err != nil {
			return fmt.Errorf("error opening default icalfile storage: %w", err)
		}
		store = icalStorage
		// Make sure to close and write the storage before exiting
		defer func() {
			if err := icalStorage.WriteAndUnlock(); err != nil {
				fmt.Fprintf(os.Stderr, "Error closing default icalfile storage: %v\n", err)
			}
		}()
	}

	// Initialize the processor
	proc := processor.NewProcessor(store, config.ProcessReplies)

	// Process maildir if specified
	if config.MaildirPath != "" {
		if err := maildir.Process(config.MaildirPath, proc, config.Verbose); err != nil {
			return fmt.Errorf("error processing maildir: %w", err)
		}
		return nil
	}

	// Default: process from stdin
	if err := stdin.Process(proc); err != nil {
		return fmt.Errorf("error processing stdin: %w", err)
	}
	
	return nil
}