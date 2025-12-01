package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// ServicesCmd is the CLI services command for managing launchd service.
var ServicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Manage the neru launchd service",
	Long:  `Manage the neru launchd service for automatic startup.`,
}

// ServicesInstallCmd is the CLI install subcommand.
var ServicesInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and load the launchd service",
	Long:  `Install the launchd service by copying the plist to LaunchAgents and loading it.`,
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
	Short: "Unload and remove the launchd service",
	Long:  `Unload the launchd service and remove the plist from LaunchAgents.`,
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
	Short: "Start the launchd service",
	Long:  `Start the neru launchd service.`,
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
	Short: "Stop the launchd service",
	Long:  `Stop the neru launchd service.`,
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
	Short: "Restart the launchd service",
	Long:  `Restart the neru launchd service.`,
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
	Short: "Check the status of the launchd service",
	Long:  `Check if the neru launchd service is loaded and running.`,
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

const (
	serviceLabel    = "com.y3owk1n.neru"
	launchAgentsDir = "~/Library/LaunchAgents"
	plistFile       = launchAgentsDir + "/" + serviceLabel + ".plist"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.y3owk1n.neru</string>
    <key>ProgramArguments</key>
    <array>
        <string>NERU_BINARY_PATH</string>
        <string>launch</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/neru.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/neru.err</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string>
    </dict>
</dict>
</plist>`

func getBinaryPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(execPath)
}

func installService() error {
	binPath, err := getBinaryPath()
	if err != nil {
		return fmt.Errorf("failed to get binary path: %w", err)
	}

	// Replace placeholder in template
	plistContent := strings.ReplaceAll(plistTemplate, "NERU_BINARY_PATH", binPath)

	// Expand launchAgentsDir
	expandedDir, err := expandPath(launchAgentsDir)
	if err != nil {
		return fmt.Errorf("failed to expand LaunchAgents path: %w", err)
	}

	// Ensure directory exists
	const dirPerm = 0o755

	err = os.MkdirAll(expandedDir, dirPerm)
	if err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	// Write plist
	expandedPlist := filepath.Join(expandedDir, serviceLabel+".plist")

	const filePerm = 0o644

	err = os.WriteFile(expandedPlist, []byte(plistContent), filePerm)
	if err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	// Load service
	cmd := exec.CommandContext(context.Background(), "launchctl", "load", expandedPlist)

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to load service: %w", err)
	}

	return nil
}

func uninstallService() error {
	expandedPlist, err := expandPath(plistFile)
	if err != nil {
		return fmt.Errorf("failed to expand plist path: %w", err)
	}

	// Unload service if loaded
	cmd := exec.CommandContext(context.Background(), "launchctl", "unload", expandedPlist)
	_ = cmd.Run() // Ignore error if not loaded

	// Remove plist
	err = os.Remove(expandedPlist)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	return nil
}

func startService() error {
	cmd := exec.CommandContext(context.Background(), "launchctl", "start", serviceLabel)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

func stopService() error {
	cmd := exec.CommandContext(context.Background(), "launchctl", "stop", serviceLabel)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	return nil
}

func restartService() error {
	// Stop the service (ignore errors if already stopped)
	_ = stopService()

	// Always attempt to start
	return startService()
}

func statusService() string {
	cmd := exec.CommandContext(context.Background(), "launchctl", "list", serviceLabel)

	_, err := cmd.Output()
	if err != nil {
		return "Service not loaded"
	}

	return "Service loaded"
}

func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		return filepath.Join(home, path[1:]), nil
	}

	return path, nil
}
