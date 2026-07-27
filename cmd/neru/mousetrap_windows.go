//go:build windows

package main

import "github.com/spf13/cobra"

func init() {
	// Disable cobra's Windows mousetrap, which shows "This is a command line
	// tool..." when double-clicking from Explorer. We handle this ourselves
	// via isRunningFromAppBundle / GetConsoleProcessList, which launches the
	// daemon instead of printing a notice.
	cobra.MousetrapHelpText = ""
}
