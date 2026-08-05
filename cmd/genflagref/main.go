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

	"github.com/y3owk1n/neru/internal/flagref"
)

func main() {
	if len(os.Args) < 2 { //nolint:mnd
		fmt.Fprintf(os.Stderr, "Usage: genflagref <markdown-file>\n")
		os.Exit(1)
	}

	path := os.Args[1]

	contents, err := os.ReadFile(path) //nolint:gosec // the path is a build argument
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", path, err)
		os.Exit(1)
	}

	updated, err := flagref.Rewrite(string(contents))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering the mode-flag reference: %v\n", err)
		os.Exit(1)
	}

	if updated == string(contents) {
		fmt.Printf("✓ %s already lists every mode flag\n", path) //nolint:forbidigo

		return
	}

	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading the mode of %s: %v\n", path, err)
		os.Exit(1)
	}

	// The page's own mode is kept: a generated region does not make the file
	// the generator's to re-permission.
	err = os.WriteFile(path, []byte(updated), info.Mode().Perm())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Mode-flag reference written to %s\n", path) //nolint:forbidigo
}
