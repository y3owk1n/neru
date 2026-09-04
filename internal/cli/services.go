package cli

import (
	"github.com/spf13/cobra"
)

// ServicesCmd is the CLI services command for managing the system service.
//
// macOS: backed by a launchd user agent.
// Linux: backed by a systemd user unit; other init systems return
// CodeNotSupported.
// Windows: backed by a Task Scheduler logon task for the current user.
// Other platforms: stubbed and returns CodeNotSupported until implemented.
var ServicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Manage the Neru system service (launchd, systemd, Task Scheduler)",
	Long: `Manage the Neru system service for automatic startup on login.

On macOS this manages a launchd agent, on Linux a systemd user unit
ordered after graphical-session.target, and on Windows a Task Scheduler
task with a logon trigger, so Neru starts automatically once your
session is up. Linux service management covers systemd only; other init
systems report ERR_NOT_SUPPORTED.

Subcommands:
  install     Install and load the system service
  uninstall   Unload and remove the system service
  start       Start the system service
  stop        Stop the system service
  restart     Restart the system service
  status      Check whether the service is loaded and running`,
}

// ServicesInstallCmd is the CLI install subcommand.
var ServicesInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and load the system service",
	Long:  `Install the Neru service so it starts automatically on login: a launchd plist loaded with launchctl on macOS, a systemd user unit enabled with systemctl --user on Linux, a Task Scheduler logon task on Windows.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := installService()
		if err != nil {
			return err
		}

		cmd.Println("Service installed and loaded successfully")

		return nil
	},
}

// ServicesUninstallCmd is the CLI uninstall subcommand.
var ServicesUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Unload and remove the system service",
	Long:  `Unload the Neru service and remove what describes it — the launchd plist on macOS, the systemd user unit on Linux, the Task Scheduler task on Windows. Neru will no longer start automatically on login.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := uninstallService()
		if err != nil {
			return err
		}

		cmd.Println("Service uninstalled successfully")

		return nil
	},
}

// ServicesStartCmd is the CLI start subcommand.
var ServicesStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the system service",
	Long:  `Start the installed Neru service. The daemon will begin running in the background.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := startService()
		if err != nil {
			return err
		}

		cmd.Println("Service started")

		return nil
	},
}

// ServicesStopCmd is the CLI stop subcommand.
var ServicesStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the system service",
	Long:  `Stop the installed Neru service. The daemon process will be terminated; the service stays installed and starts again on your next login.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := stopService()
		if err != nil {
			return err
		}

		cmd.Println("Service stopped")

		return nil
	},
}

// ServicesRestartCmd is the CLI restart subcommand.
var ServicesRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the system service",
	Long:  `Stop then immediately start the Neru service. Useful after configuration changes or to recover from an unresponsive state.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := restartService()
		if err != nil {
			return err
		}

		cmd.Println("Service restarted")

		return nil
	},
}

// ServicesStatusCmd is the CLI status subcommand.
var ServicesStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of the system service",
	Long:  `Check whether the Neru service is installed and running. A machine where it was never installed is reported as such rather than as an error.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println(statusService())

		return nil
	},
}

func init() {
	ServicesCmd.AddCommand(ServicesInstallCmd)
	ServicesCmd.AddCommand(ServicesUninstallCmd)
	ServicesCmd.AddCommand(ServicesStartCmd)
	ServicesCmd.AddCommand(ServicesStopCmd)
	ServicesCmd.AddCommand(ServicesRestartCmd)
	ServicesCmd.AddCommand(ServicesStatusCmd)
	RootCmd.AddCommand(ServicesCmd)
}
