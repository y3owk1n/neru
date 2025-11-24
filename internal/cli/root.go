package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"github.com/y3owk1n/neru/internal/infra/logger"
	"go.uber.org/zap"
)

var (
	configPath string
	// LaunchFunc is set by main to handle daemon launch.
	LaunchFunc func(configPath string)
	// Version is set via ldflags at build time.
	Version = "dev"
	// GitCommit is set via ldflags at build time.
	GitCommit = "unknown"
	// BuildDate is set via ldflags at build time.
	BuildDate  = "unknown"
	timeoutSec = 5
)

var rootCmd = &cobra.Command{
	Use:   "neru",
	Short: "Neru - Keyboard-driven navigation for macOS",
	Long: `Neru is a keyboard-driven navigation tool for macOS that provides
vim-like navigation capabilities across all applications using accessibility APIs.`,
	Version: Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isRunningFromAppBundle() && len(args) == 0 {
			logger.Info("Launching Neru from app bundle...")
			launchProgram(configPath)

			return nil
		}

		return cmd.Help()
	},
}

// Execute initializes and runs the CLI application.
func Execute() {
	executeErr := rootCmd.Execute()
	if executeErr != nil {
		fmt.Fprintln(os.Stderr, executeErr)
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

	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to config file")
	rootCmd.PersistentFlags().IntVar(&timeoutSec, "timeout", 5, "IPC timeout in seconds")
}

func isRunningFromAppBundle() bool {
	execPath, execPathErr := os.Executable()
	if execPathErr != nil {
		return false
	}

	// Resolve symlinks to get the real path
	realPath, realPathErr := filepath.EvalSymlinks(execPath)
	if realPathErr != nil {
		realPath = execPath
	}

	return strings.Contains(realPath, ".app/Contents/MacOS")
}

func launchProgram(cfgPath string) {
	logger.Debug("Launching program", zap.String("config_path", cfgPath))

	if ipc.IsServerRunning() {
		logger.Info("Neru is already running")
		os.Exit(0)
	}

	if LaunchFunc != nil {
		logger.Debug("Calling launch function")
		LaunchFunc(cfgPath)
	} else {
		logger.Error("Launch function not initialized")
		fmt.Fprintln(os.Stderr, "Error: Launch function not initialized")
		os.Exit(1)
	}
}

// sendCommand transmits a command to the running Neru daemon via IPC.
func sendCommand(action string, args []string) error {
	logger.Debug("Sending command",
		zap.String("action", action),
		zap.Strings("args", args))

	if !ipc.IsServerRunning() {
		logger.Warn("Neru is not running")

		return errors.New("neru is not running. Start it first with 'neru' or 'neru launch'")
	}

	ipcClient := ipc.NewClient()

	ipcResponse, ipcResponseErr := ipcClient.SendWithTimeout(
		ipc.Command{Action: action, Args: args},
		time.Duration(timeoutSec)*time.Second,
	)
	if ipcResponseErr != nil {
		logger.Error("Failed to send command",
			zap.String("action", action),
			zap.Error(ipcResponseErr))

		return fmt.Errorf("failed to send command: %w", ipcResponseErr)
	}

	if !ipcResponse.Success {
		logger.Warn("Command failed",
			zap.String("action", action),
			zap.String("message", ipcResponse.Message),
			zap.String("code", ipcResponse.Code))

		if ipcResponse.Code != "" {
			return fmt.Errorf("%s (code: %s)", ipcResponse.Message, ipcResponse.Code)
		}

		return fmt.Errorf("%s", ipcResponse.Message)
	}

	logger.Debug("Command succeeded",
		zap.String("action", action),
		zap.String("message", ipcResponse.Message))

	logger.Info(ipcResponse.Message)

	return nil
}

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
