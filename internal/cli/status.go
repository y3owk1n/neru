package cli

import (
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show neru status",
	Long:  `Display the current status of the neru program.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return builder.CheckRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		// Update communicator timeout to reflect current flag value
		communicator.SetTimeout(timeoutSec)

		ipcResponse, err := communicator.SendCommand("status", []string{})
		if err != nil {
			return err
		}

		if !ipcResponse.Success {
			return communicator.HandleResponse(cmd, ipcResponse)
		}

		return formatter.PrintStatus(cmd, ipcResponse.Data)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
