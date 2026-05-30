package cli

import (
	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/cli/cliutil"
)

// StatusCmd is the CLI status command.
var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Neru daemon status",
	Long: `Display whether Neru is running, the active mode, and the current
configuration state (running or paused).`,
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
