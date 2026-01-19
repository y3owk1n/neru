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

// RootCmd is the root CLI command for Neru.
var RootCmd = &cobra.Command{
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
	executeErr := RootCmd.Execute()
	if executeErr != nil {
		fmt.Fprintln(os.Stderr, executeErr)
		os.Exit(1)
	}
}

func init() {
	// Initialize CLI utilities.
	formatter = cliutil.NewOutputFormatter()

	RootCmd.SetVersionTemplate(
		fmt.Sprintf(
			"Neru version %s\nGit commit: %s\nBuild date: %s\n",
			Version,
			GitCommit,
			BuildDate,
		),
	)

	RootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to config file")
	RootCmd.PersistentFlags().
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

// BuildSimpleCommand creates a simple cobra command with the given parameters.
func BuildSimpleCommand(use, short, long string, action string) *cobra.Command {
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

// BuildActionCommand creates an action cobra command with the given parameters.
func BuildActionCommand(use, short, long string, params []string) *cobra.Command {
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

// BuildMoveMouseCommand creates a move_mouse cobra command with x and y flags.
func BuildMoveMouseCommand() *cobra.Command {
	var targetX, targetY int

	cmd := &cobra.Command{
		Use:   "move_mouse",
		Short: "Move mouse cursor to absolute position",
		Long: `Move the mouse cursor to the specified absolute position.
Coordinates are relative to the current display.`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return sendCommand(
				cmd,
				"action",
				[]string{
					"move_mouse",
					fmt.Sprintf("--x=%d", targetX),
					fmt.Sprintf("--y=%d", targetY),
				},
			)
		},
	}

	cmd.Flags().IntVar(&targetX, "x", 0, "X coordinate (pixels)")
	cmd.Flags().IntVar(&targetY, "y", 0, "Y coordinate (pixels)")
	_ = cmd.MarkFlagRequired("x")
	_ = cmd.MarkFlagRequired("y")

	return cmd
}

// BuildMoveMouseRelativeCommand creates a move_mouse_relative cobra command with deltaX and deltaY flags.
func BuildMoveMouseRelativeCommand() *cobra.Command {
	var deltaX, deltaY int

	cmd := &cobra.Command{
		Use:   "move_mouse_relative",
		Short: "Move mouse cursor relatively",
		Long: `Move the mouse cursor by the specified delta from current position.
Positive values move right/down, negative values move left/up.`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return sendCommand(
				cmd,
				"action",
				[]string{
					"move_mouse_relative",
					fmt.Sprintf("--dx=%d", deltaX),
					fmt.Sprintf("--dy=%d", deltaY),
				},
			)
		},
	}

	cmd.Flags().IntVar(&deltaX, "dx", 0, "Delta X (pixels, positive=right, negative=left)")
	cmd.Flags().IntVar(&deltaY, "dy", 0, "Delta Y (pixels, positive=down, negative=up)")
	_ = cmd.MarkFlagRequired("dx")
	_ = cmd.MarkFlagRequired("dy")

	return cmd
}
