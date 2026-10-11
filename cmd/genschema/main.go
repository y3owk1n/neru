// Package main is the entry point for the genschema command, which writes the
// JSON Schema for config.toml from the config structs.
//
// The guardrail test in internal/architecture fails while the committed schema
// is out of date, and this command brings it back.
package main

import (
	"fmt"
	"os"

	"github.com/y3owk1n/neru/internal/configschema"
)

func main() {
	if len(os.Args) < 2 { //nolint:mnd
		fmt.Fprintf(os.Stderr, "Usage: genschema <schema-file>\n")
		os.Exit(1)
	}

	out, err := configschema.Render()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering the config schema: %v\n", err)
		os.Exit(1)
	}

	err = os.WriteFile(os.Args[1], out, 0o644) //nolint:gosec,mnd // a published file
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing the config schema: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Config schema: written to %s\n", os.Args[1]) //nolint:forbidigo
}
