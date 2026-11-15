// Package main is the entry point for the genhookref command, which rewrites
// the hooks and hook-variable tables in docs/reference/configuration.md from
// the event catalog.
package main

import (
	"fmt"
	"os"

	"github.com/y3owk1n/neru/internal/docsregion"
	"github.com/y3owk1n/neru/internal/hookref"
)

func main() {
	if len(os.Args) < 2 { //nolint:mnd
		fmt.Fprintf(os.Stderr, "Usage: genhookref <markdown-file>\n")
		os.Exit(1)
	}

	result, err := docsregion.Generate(os.Args[1], hookref.Rewrite)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing the hooks reference: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Hooks reference: %s\n", result) //nolint:forbidigo
}
