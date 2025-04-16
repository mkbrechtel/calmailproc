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
	if verbose {
		entries, err := os.ReadDir(maildirPath)
		if err != nil {
			return fmt.Errorf("reading maildir: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Found %d entries in maildir root\n", len(entries))
		for _, entry := range entries {
			fmt.Fprintf(os.Stderr, "  - %s (dir: %t)\n", entry.Name(), entry.IsDir())
		}
	}

	// Process the current maildir
	if err := processMaildirDirectory(maildirPath, proc, verbose); err != nil {
		return fmt.Errorf("processing maildir %s: %w", maildirPath, err)
	}

	return nil
}

// processMaildirDirectory processes a maildir directory and all its subfolders
func processMaildirDirectory(dirPath string, proc *processor.Processor, verbose bool) error {
	// Process the standard maildir folders (new and cur)
	if err := processStandardMaildirFolders(dirPath, proc, verbose); err != nil {
		return err
	}

	// Don't process emails directly in the main directory - only standard maildir folders
	// and subdirectories will be processed

	// Process subdirectories recursively
	return processSubdirectories(dirPath, proc, verbose)
}

// processStandardMaildirFolders processes the standard 'new' and 'cur' folders of a maildir
func processStandardMaildirFolders(maildirPath string, proc *processor.Processor, verbose bool) error {
	// Process the 'new' folder if it exists
	newDir := filepath.Join(maildirPath, "new")
	if _, err := os.Stat(newDir); err == nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Processing 'new' directory: %s\n", newDir)
		}
		if err := processEmailsInDirectory(newDir, proc, verbose); err != nil {
			return fmt.Errorf("processing 'new' directory: %w", err)
		}
	} else if verbose {
		fmt.Fprintf(os.Stderr, "'new' directory does not exist: %s\n", newDir)
	}

	// Process the 'cur' folder if it exists
	curDir := filepath.Join(maildirPath, "cur")
	if _, err := os.Stat(curDir); err == nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Processing 'cur' directory: %s\n", curDir)
		}
		if err := processEmailsInDirectory(curDir, proc, verbose); err != nil {
			return fmt.Errorf("processing 'cur' directory: %w", err)
		}
	} else if verbose {
		fmt.Fprintf(os.Stderr, "'cur' directory does not exist: %s\n", curDir)
	}

	return nil
}

// processEmailsInDirectory processes all email files in a directory
func processEmailsInDirectory(dirPath string, proc *processor.Processor, verbose bool) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("reading directory %s: %w", dirPath, err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Found %d files in directory: %s\n", len(files), dirPath)
	}

	processedCount := 0
	for _, file := range files {
		if file.IsDir() {
			continue // Skip subdirectories
		}

		// Skip known non-email files
		name := file.Name()
		if name == ".DS_Store" || name == "maildirfolder" || name == ".uidvalidity" {
			continue
		}

		// Process the email file
		filePath := filepath.Join(dirPath, name)
		if err := processEmailFile(filePath, proc, verbose); err != nil && verbose {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			continue
		}

		processedCount++
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Processed %d/%d files in: %s\n", processedCount, len(files), dirPath)
	}

	return nil
}

// processEmailFile processes a single email file
func processEmailFile(filePath string, proc *processor.Processor, verbose bool) error {
	// Open the file
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", filePath, err)
	}
	defer f.Close()

	// Process the email
	msg, err := proc.ProcessEmail(f)
	if verbose || msg != "Processed E-Mail without calendar event" {
		fmt.Fprintf(os.Stdout, "%s > %s\n", filePath, msg)
	}
	if err != nil {
		return fmt.Errorf("failed to process %s: %v", filePath, err)
	}

	return nil
}

// processSubdirectories recursively processes all subdirectories
func processSubdirectories(parentDir string, proc *processor.Processor, verbose bool) error {
	if verbose {
		fmt.Fprintf(os.Stderr, "Looking for subfolders in: %s\n", parentDir)
	}

	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return fmt.Errorf("reading directory %s: %w", parentDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip special Maildir directories
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
		if _, err := os.Stat(filepath.Join(subPath, "cur")); err == nil {
			isMaildir = true
		}
		if _, err := os.Stat(filepath.Join(subPath, "new")); err == nil {
			isMaildir = true
		}

		if isMaildir && verbose {
			fmt.Fprintf(os.Stderr, "Processing maildir subfolder: %s\n", subPath)
		} else if verbose {
			fmt.Fprintf(os.Stderr, "Processing non-maildir subfolder: %s\n", subPath)
		}

		// Process this directory (whether it's a maildir or not)
		if err := processMaildirDirectory(subPath, proc, verbose); err != nil && verbose {
			fmt.Fprintf(os.Stderr, "Warning: Error processing directory %s: %v\n", subPath, err)
		}
	}

	return nil
}