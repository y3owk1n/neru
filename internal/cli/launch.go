package cli

import (
	"github.com/spf13/cobra"
)

// LaunchCmd is the CLI launch command.
var LaunchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Start the Neru daemon",
	Long: `Start the Neru daemon process.

This initializes the accessibility engine, overlay, and IPC server
to handle navigation modes and commands. Once launched, Neru runs
in the background until quit from the system tray menu.

Use 'neru stop' to pause functionality (daemon stays running).`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		launchProgram(cmd, configPath)

		return nil
	},
}

func init() {
	RootCmd.AddCommand(LaunchCmd)
}
