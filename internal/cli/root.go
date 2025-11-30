package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/cli/cliutil"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

const (
	// DefaultIPCTimeoutSeconds is the default IPC timeout in seconds.
	DefaultIPCTimeoutSeconds = 5
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

	// CLI utilities.
	formatter *cliutil.OutputFormatter
)

var rootCmd = &cobra.Command{
	Use:   "neru",
	Short: "Neru - Keyboard-driven navigation for macOS",
	Long: `Neru is a keyboard-driven navigation tool for macOS that provides
vim-like navigation capabilities across all applications using accessibility APIs.`,
	Version: Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsRunningFromAppBundle() && len(args) == 0 {
			launchProgram(cmd, configPath)

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
	// Initialize CLI utilities.
	formatter = cliutil.NewOutputFormatter()

	rootCmd.SetVersionTemplate(
		fmt.Sprintf(
			"Neru version %s\nGit commit: %s\nBuild date: %s\n",
			Version,
			GitCommit,
			BuildDate,
		),
	)

	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to config file")
	rootCmd.PersistentFlags().
		IntVar(&timeoutSec, "timeout", DefaultIPCTimeoutSeconds, "IPC timeout in seconds")
}

// IsRunningFromAppBundle checks if the executable is running from a macOS app bundle.
func IsRunningFromAppBundle() bool {
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

func launchProgram(cmd *cobra.Command, cfgPath string) {
	if ipc.IsServerRunning() {
		cmd.Println("Neru is already running")
		os.Exit(0)
	}

	if LaunchFunc != nil {
		LaunchFunc(cfgPath)
	} else {
		cmd.PrintErrln("Error: Launch function not initialized")
		os.Exit(1)
	}
}

// sendCommand transmits a command to the running Neru daemon via IPC.
func sendCommand(cmd *cobra.Command, action string, args []string) error {
	communicator := cliutil.NewIPCCommunicator(timeoutSec)

	return communicator.SendAndHandle(cmd, action, args)
}

func requiresRunningInstance() error {
	if !ipc.IsServerRunning() {
		return derrors.New(
			derrors.CodeIPCServerNotRunning,
			"neru is not running. Start it first with 'neru' or 'neru launch'",
		)
	}

	return nil
}

func buildSimpleCommand(use, short, long string, action string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return sendCommand(cmd, action, args)
		},
	}
}

func buildActionCommand(use, short, long string, params []string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return sendCommand(cmd, "action", params)
		},
	}
}
