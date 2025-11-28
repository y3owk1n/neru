package cli

import (
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the neru program (resume if paused)",
	Long:  `Start or resume the neru program. This enables neru if it was previously stopped.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return sendCommand(cmd, "start", args)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
