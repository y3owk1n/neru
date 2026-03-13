//go:build !darwin

package cli

import (
	"github.com/spf13/cobra"
)

// ServicesCmd is a stub on non-macOS platforms.
// Service management (launchd) is a macOS-only feature.
// On Linux, use your system's service manager (systemd, openrc, etc.).
// On Windows, use the Windows Service Manager or Task Scheduler.
var ServicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Manage the neru system service",
	Long: `Manage the neru system service for automatic startup.

NOTE: Automatic service management via this command is currently only
supported on macOS (launchd). On Linux/Windows, please use your
platform's native service manager:
  - Linux:   systemd (systemctl), openrc, etc.
  - Windows: Windows Service Manager or Task Scheduler`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.Println("Service management is not yet implemented on this platform.")
		cmd.Println("See 'neru services --help' for details.")

		return nil
	},
}

func init() {
	RootCmd.AddCommand(ServicesCmd)
}
