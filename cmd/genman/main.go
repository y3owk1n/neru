// Package main is the main entry point for the genman command.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra/doc"

	"github.com/y3owk1n/neru/internal/cli"
)

func main() {
	if len(os.Args) < 2 { //nolint:mnd
		fmt.Fprintf(os.Stderr, "Usage: genman <output-dir>\n")
		os.Exit(1)
	}

	outputDir := os.Args[1]
	now := time.Now()

	header := &doc.GenManHeader{
		Title:   "NERU",
		Section: "1",
		Date:    &now,
		Manual:  "Neru Manual",
		Source:  "Neru " + cli.Version,
	}

	err := doc.GenManTree(cli.RootCmd, header, outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating man pages: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Man pages generated in %s/\n", outputDir) //nolint:forbidigo
}
