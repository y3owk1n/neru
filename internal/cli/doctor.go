package cli

import (
	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/cli/cliutil"
	"github.com/y3owk1n/neru/internal/core/domain"
)

// DoctorCmd is the CLI doctor command.
var DoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the health of Neru components",
	RunE: func(cmd *cobra.Command, _ []string) error {
		communicator := cliutil.NewIPCCommunicator(timeoutSec)

		ipcResponse, err := communicator.SendCommand(domain.CommandHealth, []string{})
		if err != nil {
			return err
		}

		return formatter.PrintHealth(cmd, ipcResponse.Success, ipcResponse.Data)
	},
}

func init() {
	RootCmd.AddCommand(DoctorCmd)
}
