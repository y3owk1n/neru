package cli

import (
	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/core/domain"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the health of Neru components",
	RunE: func(cmd *cobra.Command, _ []string) error {
		// Update communicator timeout to reflect current flag value
		communicator.SetTimeout(timeoutSec)

		ipcResponse, err := communicator.SendCommand(domain.CommandHealth, []string{})
		if err != nil {
			return err
		}

		return formatter.PrintHealth(cmd, ipcResponse.Success, ipcResponse.Data)
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
