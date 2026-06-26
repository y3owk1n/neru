//go:build windows

package main

import "github.com/spf13/cobra"

func init() {
	// Disable cobra's Windows mousetrap, which shows "This is a command line
	// tool..." when double-clicking from Explorer. We handle this ourselves
	// via isRunningFromAppBundle / GetConsoleWindow.
	cobra.MousetrapHelpText = ""
}
