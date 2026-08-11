package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/cli/cliutil"
	"github.com/y3owk1n/neru/internal/domain"
)

var (
	errDaemonUnreachable = errors.New("daemon unreachable")
	// errClickableRolesUnusable marks a configuration that loads but selects no
	// accessibility role on this platform, so hints would find nothing.
	errClickableRolesUnusable = errors.New("no clickable roles apply on this platform")
)

// DoctorCmd is the CLI doctor command.
var DoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run comprehensive diagnostics",
	Long: `Run a comprehensive health check of the Neru system.

This command performs client-side checks (IPC socket, config) first,
then queries the running daemon for component-level health status
(accessibility permissions, overlay state, input monitoring).

The platform_support row is a different question from the component
health below it: it names the options, actions and mode flags your
configuration writes that do nothing on this platform. They load and
the daemon runs, so it never fails the check.

Runs client-side checks even when the daemon is not running, so you
can use it to verify accessibility permissions before launching.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.Println("Neru Doctor — pre-flight checks")
		cmd.Println()
		// --- client-side checks (no daemon needed) --------------------------
		loadResult := doctorConfigLoad()

		rolesUsable := printClickableRolesCheck(cmd, loadResult)

		printPlatformSupportCheck(cmd, loadResult)

		endpointPath := ipc.SocketPath()

		if !ipc.IsServerRunning() {
			cmd.Printf("  ❌ %-24s %s\n", "ipc_endpoint", "not reachable: "+endpointPath)

			err := printClientDoctorWithoutDaemon(cmd)
			if err != nil {
				return err
			}

			// Not every platform treats a stopped daemon as a failure, so the
			// role check still has to be able to fail this branch.
			if !rolesUsable {
				return &silentError{err: errClickableRolesUnusable}
			}

			return nil
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

		if err != nil {
			return err
		}

		// A healthy daemon running a config that selects no roles still cannot
		// produce a hint, so the client-side failure has to reach the exit
		// status or a scripted health check would treat it as fine.
		if !rolesUsable {
			return &silentError{err: errClickableRolesUnusable}
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(DoctorCmd)
}
