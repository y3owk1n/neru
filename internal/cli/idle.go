package cli

import (
	"github.com/spf13/cobra"
)

// IdleCmd is the CLI idle command.
var IdleCmd = &cobra.Command{
	Use:   "idle",
	Short: "Set mode to idle",
	Long:  `Exit the current mode and return to idle state.`,
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
