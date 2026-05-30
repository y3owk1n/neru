package cli

import (
	"github.com/spf13/cobra"
)

// IdleCmd is the CLI idle command.
var IdleCmd = &cobra.Command{
	Use:   "idle",
	Short: "Exit the current navigation mode",
	Long: `Exit the current navigation mode (hints, grid, recursive-grid, scroll)
and return to idle state. Useful for scripting mode transitions.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return sendCommand(cmd, "idle", args)
	},
}

func init() {
	RootCmd.AddCommand(IdleCmd)
}
