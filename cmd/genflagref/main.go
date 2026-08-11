// Package main is the entry point for the genflagref command, which writes the
// mode-flag reference into a documentation page from the grammar's descriptor
// table.
//
// It is the writing half of the same contract the guardrail test in
// internal/architecture reads: the test fails when a page is out of date, and
// this is what brings it back.
package main

import (
	"fmt"
	"os"

	"github.com/y3owk1n/neru/internal/docsregion"
	"github.com/y3owk1n/neru/internal/flagref"
)

func main() {
	if len(os.Args) < 2 { //nolint:mnd
		fmt.Fprintf(os.Stderr, "Usage: genflagref <markdown-file>\n")
		os.Exit(1)
	}

	result, err := docsregion.Generate(os.Args[1], flagref.Rewrite)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing the mode-flag reference: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Mode-flag reference: %s\n", result) //nolint:forbidigo
}
