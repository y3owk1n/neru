package cli

import (
	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the neru program (resume if paused)",
	Long:  `Start or resume the neru program. This enables neru if it was previously stopped.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(_ *cobra.Command, args []string) error {
		logger.Debug("Starting/resuming program")

		return sendCommand("start", args)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
