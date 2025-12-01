package cli

import (
	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/cli/cliutil"
)

// StatusCmd is the CLI status command.
var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show neru status",
	Long:  `Display the current status of the neru program.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		communicator := cliutil.NewIPCCommunicator(timeoutSec)

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
	RootCmd.AddCommand(StatusCmd)
}
