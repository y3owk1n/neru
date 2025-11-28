package cli

import (
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Pause the neru program (does not quit)",
	Long:  `Pause the neru program. This disables neru functionality but keeps it running in the background.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return sendCommand(cmd, "stop", args)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
