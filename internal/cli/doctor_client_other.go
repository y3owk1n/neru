//go:build !windows

// internal/cli/doctor_client_other.go
// Non-Windows fallback when the daemon IPC socket is missing.
// Does not run client-side platform probes.

package cli

import "github.com/spf13/cobra"

func printClientDoctorWithoutDaemon(cmd *cobra.Command) error {
	cmd.Println()
	cmd.Println("The neru daemon does not appear to be running.")
	cmd.Println("Start it with: neru launch")

	return &silentError{err: errDaemonNotRunning}
}
