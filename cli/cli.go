package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/adrg/xdg"
	"github.com/mkbrechtel/calmailproc/processor"
	"github.com/mkbrechtel/calmailproc/processor/maildir"
	"github.com/mkbrechtel/calmailproc/processor/stdin"
	"github.com/mkbrechtel/calmailproc/storage"
	"gopkg.in/yaml.v3"
)

type StdinConfig struct {
	Enabled bool `yaml:"enabled"`
}

// Config contains all the CLI configuration options
type Config struct {
	WebDAV    storage.WebdavConfig     `yaml:"webdav"`
	Processor processor.ProcessorConfig `yaml:"processor"`
	Maildir   maildir.MaildirConfig    `yaml:"maildir"`
	Stdin     StdinConfig              `yaml:"stdin"`

	ProcessReplies bool
	URL            string
	User           string
	Pass           string
	Calendar       string
	MaildirPath    string
	Verbose        bool
}

func loadConfigFile() (*Config, error) {
	configPath, err := xdg.ConfigFile("calmailproc/config.yaml")
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &Config{}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func ParseFlags() *Config {
	config, err := loadConfigFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config file: %v\n", err)
		config = &Config{}
	}

	flag.BoolVar(&config.ProcessReplies, "process-replies", config.Processor.ProcessReplies, "Process attendance replies to update events")

	flag.StringVar(&config.URL, "url", config.WebDAV.URL, "CalDAV server URL (e.g., http://localhost:5232)")
	flag.StringVar(&config.User, "user", config.WebDAV.User, "CalDAV username")
	flag.StringVar(&config.Pass, "pass", config.WebDAV.Pass, "CalDAV password")
	flag.StringVar(&config.Calendar, "calendar", config.WebDAV.Calendar, "CalDAV calendar path (e.g., /calendar/)")

	flag.StringVar(&config.MaildirPath, "maildir", config.Maildir.Path, "Path to maildir to process (will process all emails recursively)")
	flag.BoolVar(&config.Verbose, "verbose", config.Maildir.Verbose, "Enable verbose logging output")

	flag.Parse()

	if config.URL != "" {
		config.WebDAV.URL = config.URL
	}
	if config.User != "" {
		config.WebDAV.User = config.User
	}
	if config.Pass != "" {
		config.WebDAV.Pass = config.Pass
	}
	if config.Calendar != "" {
		config.WebDAV.Calendar = config.Calendar
	}
	if config.ProcessReplies {
		config.Processor.ProcessReplies = config.ProcessReplies
	}
	if config.MaildirPath != "" {
		config.Maildir.Path = config.MaildirPath
	}
	if config.Verbose {
		config.Maildir.Verbose = config.Verbose
	}

	return config
}

func Run(config *Config) error {
	if config.WebDAV.URL == "" || config.WebDAV.User == "" || config.WebDAV.Pass == "" || config.WebDAV.Calendar == "" {
		return fmt.Errorf("all CalDAV flags are required: -url, -user, -pass, -calendar")
	}

	store, err := storage.NewCalDAVStorageFromConfig(config.WebDAV)
	if err != nil {
		return fmt.Errorf("error initializing CalDAV storage: %w", err)
	}

	proc := processor.NewProcessorFromConfig(store, config.Processor)

	if config.Maildir.Path != "" {
		if err := maildir.ProcessWithConfig(config.Maildir, proc); err != nil {
			return fmt.Errorf("error processing maildir: %w", err)
		}
		return nil
	}

	if err := stdin.Process(proc); err != nil {
		return fmt.Errorf("error processing stdin: %w", err)
	}

	return nil
}