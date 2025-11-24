package cli

import (
	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Pause the neru program (does not quit)",
	Long:  `Pause the neru program. This disables neru functionality but keeps it running in the background.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(_ *cobra.Command, args []string) error {
		logger.Debug("Stopping/pausing program")

		return sendCommand("stop", args)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
