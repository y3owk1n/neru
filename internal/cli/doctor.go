package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/cli/cliutil"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

var errDaemonUnreachable = errors.New("daemon unreachable")

// DoctorCmd is the CLI doctor command.
var DoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run comprehensive diagnostics",
	Long: `Run a comprehensive health check of the Neru system.

This command performs client-side checks (IPC socket, config) first,
then queries the running daemon for component-level health status
(accessibility permissions, overlay state, input monitoring).

Runs client-side checks even when the daemon is not running, so you
can use it to verify accessibility permissions before launching.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.Println("Neru Doctor — pre-flight checks")
		cmd.Println()
		// --- client-side checks (no daemon needed) --------------------------
		endpointPath := ipc.SocketPath()

		if !ipc.IsServerRunning() {
			cmd.Printf("  ❌ %-24s %s\n", "ipc_endpoint", "not reachable: "+endpointPath)

			return printClientDoctorWithoutDaemon(cmd)
		}

		cmd.Printf("  ✅ %-24s %s\n", "ipc_endpoint", endpointPath)
		cmd.Println()
		// --- daemon-side checks (via IPC) -----------------------------------
		cmd.Println("Querying daemon...")
		cmd.Println()

		communicator := cliutil.NewIPCCommunicator(timeoutSec)

		ipcResponse, err := communicator.SendCommand(domain.CommandHealth, []string{})
		if err != nil {
			cmd.Printf("  ❌ %-24s %s\n", "daemon", "unreachable")
			cmd.Println()
			cmd.Println("The daemon endpoint exists but is not responding.")
			cmd.Println("Try restarting: neru launch")

			return &silentError{err: errDaemonUnreachable}
		}

		err = formatter.PrintHealth(cmd, ipcResponse.Success, ipcResponse.Data)

		if errors.Is(err, cliutil.ErrUnhealthy) {
			return &silentError{err: err}
		}

		return err
	},
}

func init() {
	RootCmd.AddCommand(DoctorCmd)
}
