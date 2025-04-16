package stdin

import (
	"fmt"
	"io"
	"os"

	"github.com/mkbrechtel/calmailproc/processor"
)

// Process processes a single email from stdin
func Process(proc *processor.Processor) error {
	// Process email from stdin
	msg, err := proc.ProcessEmail(os.Stdin)
	if err != nil {
		return fmt.Errorf("processing stdin: %w", err)
	}
	fmt.Println(msg)
	return nil
}

// ProcessReader processes a single email from an io.Reader
// Useful for testing and for cases where the input isn't strictly stdin
func ProcessReader(r io.Reader, proc *processor.Processor) error {
	// Process email from reader
	msg, err := proc.ProcessEmail(r)
	if err != nil {
		return fmt.Errorf("processing email: %w", err)
	}
	fmt.Println(msg)
	return nil
}