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
configuration state (running or paused).

Use --json to print the same information as a JSON object, for scripts and
other tools that drive Neru.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		asJSON, flagErr := cmd.Flags().GetBool("json")
		if flagErr != nil {
			return flagErr
		}

		communicator := cliutil.NewIPCCommunicator(timeoutSec)

		ipcResponse, err := communicator.SendCommand("status", []string{})
		if err != nil {
			return err
		}

		if !ipcResponse.Success {
			return communicator.HandleResponse(cmd, ipcResponse)
		}

		if asJSON {
			return formatter.PrintJSON(cmd, ipcResponse.Data)
		}

		return formatter.PrintStatus(cmd, ipcResponse.Data)
	},
}

func init() {
	StatusCmd.Flags().Bool("json", false, "Print the status as a JSON object")

	RootCmd.AddCommand(StatusCmd)
}
