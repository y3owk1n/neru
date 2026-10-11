// Package main is the entry point for the genschema command, which writes the
// JSON Schemas Neru publishes: config.toml's from the config structs, and the
// IPC wire format's from the types the daemon encodes.
//
// The guardrail tests in internal/architecture fail while a committed schema
// is out of date, and this command brings them back.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/y3owk1n/neru/internal/configschema"
	"github.com/y3owk1n/neru/internal/ipcschema"
)

func main() {
	if len(os.Args) < 2 { //nolint:mnd
		fmt.Fprintf(os.Stderr, "Usage: genschema <repository-root>\n")
		os.Exit(1)
	}

	schemas := []struct {
		path   string
		render func() ([]byte, error)
	}{
		{configschema.Path, configschema.Render},
		{ipcschema.Path, ipcschema.Render},
	}

	for _, schema := range schemas {
		out, err := schema.render()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error rendering %s: %v\n", schema.path, err)
			os.Exit(1)
		}

		path := filepath.Join(os.Args[1], schema.path)

		err = os.WriteFile(path, out, 0o644) //nolint:gosec,mnd // a published file
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
			os.Exit(1)
		}

		fmt.Printf("✓ Schema: written to %s\n", path) //nolint:forbidigo
	}
}
