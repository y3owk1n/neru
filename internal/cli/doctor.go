package cli

import (
	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the health of Neru components",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if !ipc.IsServerRunning() {
			cmd.Println("❌ Neru is not running")

			return nil
		}

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
