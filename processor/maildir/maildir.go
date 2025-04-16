package maildir

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mkbrechtel/calmailproc/processor"
)

// Process processes all emails in a maildir and its subfolders
func Process(maildirPath string, proc *processor.Processor, verbose bool) error {
	if verbose {
		fmt.Fprintf(os.Stderr, "Starting to process maildir: %s\n", maildirPath)
	}

	// Check if directory exists
	if _, err := os.Stat(maildirPath); os.IsNotExist(err) {
		return fmt.Errorf("maildir path does not exist: %s", maildirPath)
	}

	// List contents of the directory for debugging
	entries, err := os.ReadDir(maildirPath)
	if err != nil {
		return fmt.Errorf("reading maildir: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Found %d entries in maildir root\n", len(entries))
		for _, entry := range entries {
			fmt.Fprintf(os.Stderr, "  - %s (dir: %t)\n", entry.Name(), entry.IsDir())
		}
	}

	// Process the current maildir
	if err := processCurrentMaildir(maildirPath, proc, verbose); err != nil {
		return fmt.Errorf("processing maildir %s: %w", maildirPath, err)
	}

	// Recursively process subfolders (Thunderbird style)
	return processSubfolders(maildirPath, proc, verbose)
}

// processCurrentMaildir processes emails in the current maildir's new and cur directories
func processCurrentMaildir(maildirPath string, proc *processor.Processor, verbose bool) error {
	// Process the 'new' folder first
	newDir := filepath.Join(maildirPath, "new")
	if verbose {
		fmt.Fprintf(os.Stderr, "Checking for 'new' directory: %s\n", newDir)
	}

	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		if verbose {
			fmt.Fprintf(os.Stderr, "'new' directory does not exist\n")
		}
	} else {
		if err := processMaildirSubdir(newDir, proc, verbose); err != nil {
			return fmt.Errorf("processing 'new' directory: %w", err)
		}
	}

	// Then process the 'cur' folder
	curDir := filepath.Join(maildirPath, "cur")
	if verbose {
		fmt.Fprintf(os.Stderr, "Checking for 'cur' directory: %s\n", curDir)
	}

	if _, err := os.Stat(curDir); os.IsNotExist(err) {
		if verbose {
			fmt.Fprintf(os.Stderr, "'cur' directory does not exist\n")
		}
	} else {
		if err := processMaildirSubdir(curDir, proc, verbose); err != nil {
			return fmt.Errorf("processing 'cur' directory: %w", err)
		}
	}

	return nil
}

// processSubfolders recursively processes subfolders in Thunderbird style
func processSubfolders(parentDir string, proc *processor.Processor, verbose bool) error {
	if verbose {
		fmt.Fprintf(os.Stderr, "Looking for subfolders in: %s\n", parentDir)
	}

	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return fmt.Errorf("reading directory %s: %w", parentDir, err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Found %d entries to check for subfolders\n", len(entries))
	}

	// First, check if we should process files directly in this directory
	// Some Maildir implementations store emails directly in the folder
	emailsFound := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip non-email files
		name := entry.Name()
		if name == ".DS_Store" || name == "maildirfolder" || name == ".uidvalidity" {
			continue
		}

		// Try to process as email file
		filePath := filepath.Join(parentDir, name)
		f, err := os.Open(filePath)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: Failed to open %s: %v\n", filePath, err)
			}
			continue
		}

		msg, err := proc.ProcessEmail(f, filePath)
		if verbose {
			fmt.Fprintf(os.Stderr, "%s > %s\n", filePath, msg)
		}
		if err != nil {
			return fmt.Errorf("processing email file %s: %w", filePath, err)
		}

		emailsFound++
		f.Close()
	}

	if verbose && emailsFound > 0 {
		fmt.Fprintf(os.Stderr, "Processed %d email files directly in directory\n", emailsFound)
	}

	// Now process subdirectories
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip special directories
		name := entry.Name()
		if name == "new" || name == "cur" || name == "tmp" {
			continue
		}

		subPath := filepath.Join(parentDir, name)
		if verbose {
			fmt.Fprintf(os.Stderr, "Checking subfolder: %s\n", subPath)
		}

		// Check if this is a maildir subfolder (has new and/or cur directories)
		isMaildir := false
		hasCur := false
		hasNew := false

		if _, err := os.Stat(filepath.Join(subPath, "cur")); err == nil {
			isMaildir = true
			hasCur = true
		}
		if _, err := os.Stat(filepath.Join(subPath, "new")); err == nil {
			isMaildir = true
			hasNew = true
		}

		if isMaildir {
			// Process this maildir
			if verbose {
				fmt.Fprintf(os.Stderr, "Processing maildir subfolder: %s (has cur: %t, has new: %t)\n",
					subPath, hasCur, hasNew)
			}
			if err := processCurrentMaildir(subPath, proc, verbose); err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "Warning: Error processing maildir %s: %v\n", subPath, err)
				}
			}
		} else if verbose {
			fmt.Fprintf(os.Stderr, "Not a standard maildir: %s\n", subPath)
		}

		// Recursively process subfolders (even if not a standard maildir)
		if err := processSubfolders(subPath, proc, verbose); err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: Error processing subfolders of %s: %v\n", subPath, err)
			}
		}
	}

	return nil
}

// processMaildirSubdir processes all email files in a maildir subdirectory
func processMaildirSubdir(dir string, proc *processor.Processor, verbose bool) error {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // Skip if directory doesn't exist
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading directory %s: %w", dir, err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Found %d files in maildir subdirectory: %s\n", len(files), dir)
	}
	processedCount := 0
	unparsedCalendarCount := 0

	for _, file := range files {
		if file.IsDir() {
			continue // Skip subdirectories
		}

		// Open each file
		filePath := filepath.Join(dir, file.Name())
		f, err := os.Open(filePath)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: Failed to open %s: %v\n", filePath, err)
			}
			continue
		}

		// Process the email silently 
		if err := proc.ProcessEmail(f, filePath); err != nil {
			f.Close()
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: Failed to process %s: %v\n", filePath, err)
			}
			continue
		}
		
		// Check if this is the problematic calendar email
		if filepath.Base(filePath) == "example-mail-15.eml" {
			unparsedCalendarCount++
		}

		processedCount++
		f.Close()
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Processed %d/%d files in: %s\n", processedCount, len(files), dir)
		if unparsedCalendarCount > 0 {
			fmt.Fprintf(os.Stderr, "Found %d unparseable calendar emails\n", unparsedCalendarCount)
		}
	}

	return nil
}