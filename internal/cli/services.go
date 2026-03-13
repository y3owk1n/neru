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
	Short: "Manage the neru system service",
	Long:  `Manage the neru system service for automatic startup.`,
}

// ServicesInstallCmd is the CLI install subcommand.
var ServicesInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and load the system service",
	Long:  `Install the system service by placing its config and loading it.`,
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
	Long:  `Unload the system service and remove its config.`,
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
	Long:  `Start the neru system service.`,
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
	Long:  `Stop the neru system service.`,
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
	Long:  `Restart the neru system service.`,
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
	Long:  `Check if the neru system service is loaded and running.`,
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
