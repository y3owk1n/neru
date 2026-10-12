//go:build !windows

package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// stoppedDaemonFailsDoctor is whether `neru doctor` fails while the daemon is
// stopped. Here it does, since there are no client-side probes to run instead.
const stoppedDaemonFailsDoctor = true

// Non-Windows fallback when the daemon IPC socket is missing.
// Does not run client-side platform probes.
var errDaemonNotRunning = errors.New("daemon not running")

func printClientDoctorWithoutDaemon(cmd *cobra.Command) error {
	cmd.Println()
	cmd.Println("The neru daemon does not appear to be running.")
	cmd.Println("Start it with: neru launch")

	return &silentError{err: errDaemonNotRunning}
}
