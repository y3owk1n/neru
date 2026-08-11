// Package main is the entry point for the gensupportref command, which writes
// the platform-support table into a documentation page from the declarations
// that own the words.
//
// It is the writing half of the same contract the guardrail test in
// internal/architecture reads: the test fails when a page is out of date, and
// this is what brings it back.
package main

import (
	"fmt"
	"os"

	"github.com/y3owk1n/neru/internal/docsregion"
	"github.com/y3owk1n/neru/internal/supportref"
)

func main() {
	if len(os.Args) < 2 { //nolint:mnd
		fmt.Fprintf(os.Stderr, "Usage: gensupportref <markdown-file>\n")
		os.Exit(1)
	}

	result, err := docsregion.Generate(os.Args[1], supportref.Rewrite)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing the platform-support table: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Platform-support table: %s\n", result) //nolint:forbidigo
}
