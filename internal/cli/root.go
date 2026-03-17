package cli

import (
	"errors"
	"fmt"
	"os"

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
	Short: "Neru - Keyboard-driven navigation tool",
	Long: `Neru is a keyboard-driven navigation tool that provides
vim-like navigation capabilities across applications using accessibility APIs.`,
	SilenceErrors: true,
	Version:       Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsRunningFromAppBundle() && len(args) == 0 {
			launchProgram(cmd, configPath)

			return nil
		}

		return cmd.Help()
	},
}

// silentError wraps an error whose message has already been printed to stderr.
// Execute() recognizes this type and skips duplicate output.
type silentError struct{ err error }

func (e *silentError) Error() string { return e.err.Error() }
func (e *silentError) Unwrap() error { return e.err }

// setSilenceUsage walks the command tree and installs a PersistentPreRunE
// wrapper on every command that sets SilenceUsage = true.  Because the
// wrapper is attached directly to each command it cannot be silently
// shadowed by a subcommand that defines its own PersistentPreRun(E) —
// unlike a single PersistentPreRun on the root which Cobra would skip
// whenever a child overrides the hook.
func setSilenceUsage(cmd *cobra.Command) {
	origE := cmd.PersistentPreRunE
	origNonE := cmd.PersistentPreRun

	// Clear the non-E variant so Cobra never double-calls it; the
	// wrapper below invokes it when needed.
	cmd.PersistentPreRun = nil

	cmd.PersistentPreRunE = func(_cmd *cobra.Command, args []string) error {
		_cmd.SilenceUsage = true
		if origE != nil {
			return origE(_cmd, args)
		}
		// Preserve a non-E hook if one was set instead.
		if origNonE != nil {
			origNonE(_cmd, args)
		}

		return nil
	}
	for _, child := range cmd.Commands() {
		setSilenceUsage(child)
	}
}

// Execute initializes and runs the CLI application.
func Execute() {
	setSilenceUsage(RootCmd)

	executeErr := RootCmd.Execute()
	if executeErr != nil {
		// If the command already printed detailed output, don't repeat it.
		var silent *silentError
		if !errors.As(executeErr, &silent) {
			fmt.Fprintln(os.Stderr, executeErr)
		}

		os.Exit(1)
	}
}

func init() {
	// Set the build version for IPC version validation so both the CLI
	// client and the daemon (which also imports this package) agree on
	// the expected version.
	ipc.SetBuildVersion(Version)

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

// IsRunningFromAppBundle reports whether the executable is running from a
// platform app bundle. On macOS this checks for ".app/Contents/MacOS".
// On other platforms this always returns false (no bundle concept).
// Implementation is in root_darwin.go / root_other.go.
func IsRunningFromAppBundle() bool {
	return isRunningFromAppBundle()
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
	var modifier string

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			args := make([]string, 0, len(params)+1)
			args = append(args, params...)

			if modifier != "" {
				args = append(args, "--modifier="+modifier)
			}

			return sendCommand(cmd, "action", args)
		},
	}

	cmd.Flags().StringVar(&modifier, "modifier", "",
		"Comma-separated modifier keys to hold during action (cmd, shift, alt, option, ctrl)")

	return cmd
}

// BuildMoveMouseCommand creates a move_mouse cobra command with x and y flags.
func BuildMoveMouseCommand() *cobra.Command {
	var (
		targetX, targetY int
		center           bool
		monitor          string
	)

	cmd := &cobra.Command{
		Use:   "move_mouse",
		Short: "Move mouse cursor to absolute position",
		Long: `Move the mouse cursor to the specified absolute position.
Coordinates are relative to the current display.
When --center is used, the cursor moves to the center of the active screen.
If --x and --y are also provided with --center, they act as offsets from center.
When --monitor is used with --center, the cursor moves to the center of the
named monitor instead of the active screen. Monitor names are matched
case-insensitively against the localized display names reported by macOS
(e.g. "Built-in Retina Display", "DELL U2720Q").`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			hasMonitor := cmd.Flags().Changed("monitor")

			if hasMonitor && monitor == "" {
				return derrors.New(
					derrors.CodeInvalidInput,
					"--monitor value must not be empty",
				)
			}

			if hasMonitor && !center {
				return derrors.New(
					derrors.CodeInvalidInput,
					"--monitor requires --center",
				)
			}

			if !center && (!cmd.Flags().Changed("x") || !cmd.Flags().Changed("y")) {
				return derrors.New(
					derrors.CodeInvalidInput,
					"both --x and --y are required when --center is not used",
				)
			}

			args := []string{"move_mouse"}

			if center {
				args = append(args, "--center")
			}

			if hasMonitor {
				args = append(args, "--monitor="+monitor)
			}

			if cmd.Flags().Changed("x") {
				args = append(args, fmt.Sprintf("--x=%d", targetX))
			}

			if cmd.Flags().Changed("y") {
				args = append(args, fmt.Sprintf("--y=%d", targetY))
			}

			return sendCommand(cmd, "action", args)
		},
	}

	cmd.Flags().
		IntVar(&targetX, "x", 0, "X coordinate (pixels); with --center, horizontal offset (default 0)")
	cmd.Flags().
		IntVar(&targetY, "y", 0, "Y coordinate (pixels); with --center, vertical offset (default 0)")
	cmd.Flags().BoolVar(&center, "center", false, "Move to the center of the active screen")
	cmd.Flags().StringVar(&monitor, "monitor", "",
		"Target monitor by display name (requires --center); e.g. \"Built-in Retina Display\"")

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
