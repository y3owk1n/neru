package cli

import (
	"github.com/spf13/cobra"
)

var scrollCmd = &cobra.Command{
	Use:   "scroll",
	Short: "Launch scroll mode",
	Long:  `Activate scroll mode for vim-style scrolling at the cursor position.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		var params []string
		params = append(params, "scroll")

		return sendCommand(cmd, "scroll", params)
	},
}

func init() {
	rootCmd.AddCommand(scrollCmd)
}
