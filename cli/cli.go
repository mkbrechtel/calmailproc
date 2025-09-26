package cli

import (
	"flag"
	"fmt"

	"github.com/mkbrechtel/calmailproc/processor"
	"github.com/mkbrechtel/calmailproc/processor/maildir"
	"github.com/mkbrechtel/calmailproc/processor/stdin"
	"github.com/mkbrechtel/calmailproc/storage"
)

// Config contains all the CLI configuration options
type Config struct {
	ProcessReplies bool
	URL            string
	User           string
	Pass           string
	Calendar       string
	MaildirPath    string
	Verbose        bool
}

// ParseFlags parses command line flags and returns a Config
func ParseFlags() *Config {
	config := &Config{}

	// Define flags
	flag.BoolVar(&config.ProcessReplies, "process-replies", false, "Process attendance replies to update events")

	// CalDAV options
	flag.StringVar(&config.URL, "url", "", "CalDAV server URL (e.g., http://localhost:5232)")
	flag.StringVar(&config.User, "user", "", "CalDAV username")
	flag.StringVar(&config.Pass, "pass", "", "CalDAV password")
	flag.StringVar(&config.Calendar, "calendar", "", "CalDAV calendar path (e.g., /calendar/)")

	// Input options
	flag.StringVar(&config.MaildirPath, "maildir", "", "Path to maildir to process (will process all emails recursively)")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging output")

	flag.Parse()
	return config
}

// Run executes the application with the given configuration
func Run(config *Config) error {
	if config.URL == "" || config.User == "" || config.Pass == "" || config.Calendar == "" {
		return fmt.Errorf("all CalDAV flags are required: -url, -user, -pass, -calendar")
	}

	store, err := storage.NewCalDAVStorage(config.URL, config.User, config.Pass, config.Calendar)
	if err != nil {
		return fmt.Errorf("error initializing CalDAV storage: %w", err)
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