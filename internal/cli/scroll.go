package cli

import (
	"github.com/spf13/cobra"
)

// ScrollCmd is the CLI scroll command.
var ScrollCmd = &cobra.Command{
	Use:   "scroll",
	Short: "Launch scroll mode",
	Long:  `Activate scroll mode for vim-style scrolling at the cursor position.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		return sendCommand(cmd, "scroll", []string{})
	},
}

func init() {
	RootCmd.AddCommand(ScrollCmd)
}
