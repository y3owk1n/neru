package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/ipc"
	"github.com/y3owk1n/neru/internal/logger"
	"go.uber.org/zap"
)

var (
	configPath string
	// LaunchFunc is set by main to handle daemon launch.
	LaunchFunc func(configPath string)
	// Version information (set via ldflags at build time).
	Version = "dev"
	// GitCommit represents the git commit hash of the build.
	GitCommit = "unknown"
	// BuildDate represents the build date.
	BuildDate = "unknown"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "neru",
	Short: "Neru - Keyboard-driven navigation for macOS",
	Long: `Neru is a keyboard-driven navigation tool for macOS that provides
vim-like navigation capabilities across all applications using accessibility APIs.`,
	Version: Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Auto-launch when running from app bundle without arguments
		if isRunningFromAppBundle() && len(args) == 0 {
			logger.Info("Launching Neru from app bundle...")
			launchProgram(configPath)
			return nil
		}
		return cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetVersionTemplate(
		fmt.Sprintf(
			"Neru version %s\nGit commit: %s\nBuild date: %s\n",
			Version,
			GitCommit,
			BuildDate,
		),
	)
}

// isRunningFromAppBundle checks if the binary is running from a macOS app bundle.
func isRunningFromAppBundle() bool {
	execPath, err := os.Executable()
	if err != nil {
		return false
	}

	// Resolve symlinks to get the real path
	realPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		realPath = execPath
	}

	// Check if running from .app/Contents/MacOS/
	return strings.Contains(realPath, ".app/Contents/MacOS")
}

// launchProgram launches the main neru program.
func launchProgram(cfgPath string) {
	logger.Debug("Launching program", zap.String("config_path", cfgPath))

	// Check if already running
	if ipc.IsServerRunning() {
		logger.Info("Neru is already running")
		logger.Info("Neru is already running")
		os.Exit(0)
	}

	// Call the launch function set by main
	if LaunchFunc != nil {
		logger.Debug("Calling launch function")
		LaunchFunc(cfgPath)
	} else {
		logger.Error("Launch function not initialized")
		fmt.Fprintln(os.Stderr, "Error: Launch function not initialized")
		os.Exit(1)
	}
}

// sendCommand sends a command to the running neru instance.
func sendCommand(action string, args []string) error {
	logger.Debug("Sending command",
		zap.String("action", action),
		zap.Strings("args", args))

	if !ipc.IsServerRunning() {
		logger.Warn("Neru is not running")
		return errors.New("neru is not running. Start it first with 'neru' or 'neru launch'")
	}

	client := ipc.NewClient()

	response, err := client.Send(ipc.Command{Action: action, Args: args})
	if err != nil {
		logger.Error("Failed to send command",
			zap.String("action", action),
			zap.Error(err))
		return fmt.Errorf("failed to send command: %w", err)
	}

	if !response.Success {
		logger.Warn("Command failed",
			zap.String("action", action),
			zap.String("message", response.Message))
		return fmt.Errorf("%s", response.Message)
	}

	logger.Debug("Command succeeded",
		zap.String("action", action),
		zap.String("message", response.Message))

	logger.Info(response.Message)
	return nil
}

// requiresRunningInstance checks if neru is running and exits with error if not.
func requiresRunningInstance() error {
	logger.Debug("Checking if Neru is running")
	if !ipc.IsServerRunning() {
		logger.Warn("Neru is not running")
		logger.Error("Error: neru is not running")
		logger.Info("Start it first with: neru launch")
		os.Exit(1)
	}

	logger.Debug("Neru is running")
	return nil
}
