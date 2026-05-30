package cli

import (
	"github.com/spf13/cobra"
)

// ServicesCmd is the CLI services command for managing the system service.
//
// macOS: backed by launchd.
// Other platforms: stubbed and returns CodeNotSupported until implemented.
var ServicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Manage the Neru system service (macOS launchd)",
	Long: `Manage the Neru system service for automatic startup on login.

On macOS, this manages a launchd plist so Neru starts automatically
when you log in. Available on macOS only.

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
	Long:  `Install the Neru launchd service so it starts automatically on login. Creates the plist file and loads it with launchctl.`,
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
	Long:  `Unload the Neru launchd service and remove its plist file. Neru will no longer start automatically on login.`,
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
	Long:  `Start the Neru launchd service (loads the plist with launchctl). The daemon will begin running in the background.`,
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
	Long:  `Stop the Neru launchd service (unloads the plist with launchctl). The daemon process will be terminated.`,
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
	Long:  `Stop then immediately start the Neru launchd service. Useful after configuration changes or to recover from an unresponsive state.`,
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
	Long:  `Check whether the Neru launchd service is currently loaded and running. Displays the service PID if active.`,
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
