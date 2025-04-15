package main

import (
	"fmt"
	"os"

	"github.com/mkbrechtel/calmailproc/cli"
)

func main() {
	config := cli.ParseFlags()
	
	if err := cli.Run(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}