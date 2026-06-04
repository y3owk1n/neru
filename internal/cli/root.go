package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
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
	Short: "Neru - Keyboard-driven navigation tool",
	Long: `Neru is a keyboard-driven navigation tool that provides
vim-like navigation capabilities across applications using accessibility APIs.`,
	Example: `  neru launch                        Start the Neru daemon
  neru status                        Show daemon status
  neru hints --action left_click     Activate hints mode with pending click
  neru action scroll_down --steps 3  Scroll down 3 steps`,
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
		if _, ok := errors.AsType[*silentError](executeErr); !ok {
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
func BuildActionCommand(
	use, short, long string,
	params []string,
	allowTargetOverride bool,
) *cobra.Command {
	var (
		modifier  string
		selection bool
		bare      bool
	)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if selection && bare {
				return derrors.New(
					derrors.CodeInvalidInput,
					"--selection and --bare cannot be used together",
				)
			}

			args := make([]string, 0, len(params)+1)
			args = append(args, params...)

			if modifier != "" {
				args = append(args, "--modifier="+modifier)
			}

			if selection {
				args = append(args, "--selection")
			}

			if bare {
				args = append(args, "--bare")
			}

			return sendCommand(cmd, "action", args)
		},
	}

	cmd.Flags().StringVar(&modifier, "modifier", "",
		"Comma-separated modifier keys to hold during action (cmd, super, meta, shift, alt, option, ctrl)")

	if allowTargetOverride {
		cmd.Flags().BoolVar(
			&selection,
			"selection",
			false,
			"Explicitly use the active mode selection as the target point",
		)
		cmd.Flags().BoolVar(
			&bare,
			"bare",
			false,
			"Use the current cursor position even when a mode selection exists",
		)
	}

	return cmd
}

// BuildMoveMouseCommand creates a move_mouse cobra command with x and y flags.
func BuildMoveMouseCommand() *cobra.Command {
	var (
		targetX, targetY int
		center           bool
		window           bool
		selection        bool
		bare             bool
	)

	cmd := &cobra.Command{
		Use:   "move_mouse",
		Short: "Move mouse cursor to absolute position",
		Long: `Move the mouse cursor to the specified absolute position.
Coordinates are relative to the current display.
When --center is used, the cursor moves to the center of the active screen.
When --window is used, the cursor moves to the center of the focused window.
If --x and --y are also provided with --center or --window, they act as offsets from center.
Without coordinates, move_mouse targets the active mode selection by default when one exists.
Use --bare to force current-cursor targeting.`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if selection &&
				(center || window || cmd.Flags().Changed("x") || cmd.Flags().Changed("y")) {
				return derrors.New(
					derrors.CodeInvalidInput,
					"--selection cannot be combined with --x, --y, --center, or --window",
				)
			}

			if selection && bare {
				return derrors.New(
					derrors.CodeInvalidInput,
					"--selection and --bare cannot be used together",
				)
			}

			if center && window {
				return derrors.New(
					derrors.CodeInvalidInput,
					"--center and --window cannot be used together",
				)
			}

			if !center && !window && !selection &&
				((cmd.Flags().Changed("x") && !cmd.Flags().Changed("y")) ||
					(!cmd.Flags().Changed("x") && cmd.Flags().Changed("y"))) {
				return derrors.New(
					derrors.CodeInvalidInput,
					"both --x and --y are required when using absolute coordinates",
				)
			}

			args := []string{"move_mouse"}

			if center {
				args = append(args, "--center")
			}

			if window {
				args = append(args, "--window")
			}

			if cmd.Flags().Changed("x") {
				args = append(args, fmt.Sprintf("--x=%d", targetX))
			}

			if cmd.Flags().Changed("y") {
				args = append(args, fmt.Sprintf("--y=%d", targetY))
			}

			if selection {
				args = append(args, "--selection")
			}

			if bare {
				args = append(args, "--bare")
			}

			return sendCommand(cmd, "action", args)
		},
	}

	cmd.Flags().
		IntVar(&targetX, "x", 0, "X coordinate (pixels); with --center, horizontal offset (default 0)")
	cmd.Flags().
		IntVar(&targetY, "y", 0, "Y coordinate (pixels); with --center, vertical offset (default 0)")
	cmd.Flags().BoolVar(&center, "center", false, "Move to the center of the active screen")
	cmd.Flags().
		BoolVar(&window, "window", false, "Move to the center of the focused window")
	cmd.Flags().
		BoolVar(&selection, "selection", false, "Explicitly move to the active mode selection")
	cmd.Flags().
		BoolVar(&bare, "bare", false, "Use the current cursor position when no explicit target is provided")

	return cmd
}

// BuildScrollActionCommand creates a scroll action cobra command.
// If supportSteps is true, a --steps flag is added to override the scroll step amount.
func BuildScrollActionCommand(use, short, long string, supportSteps bool) *cobra.Command {
	var (
		selection bool
		bare      bool
		steps     int
	)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if selection && bare {
				return derrors.New(
					derrors.CodeInvalidInput,
					"--selection and --bare cannot be used together",
				)
			}

			args := []string{use}
			if selection {
				args = append(args, "--selection")
			}

			if bare {
				args = append(args, "--bare")
			}

			if supportSteps {
				if cmd.Flags().Changed("steps") && steps <= 0 {
					return derrors.New(
						derrors.CodeInvalidInput,
						"--steps must be a positive integer",
					)
				}

				if steps > 0 {
					args = append(args, "--steps", strconv.Itoa(steps))
				}
			}

			return sendCommand(cmd, "action", args)
		},
	}

	cmd.Flags().
		BoolVar(&selection, "selection", false, "Explicitly use the active mode selection as the target point")
	cmd.Flags().
		BoolVar(&bare, "bare", false, "Use the current cursor position even when a mode selection exists")

	if supportSteps {
		cmd.Flags().
			IntVar(&steps, "steps", 0, "Override the scroll step amount (pixels); configured default is used when omitted")
	}

	return cmd
}

// BuildMoveMonitorCommand creates a move_monitor cobra command that moves the
// cursor (and any active overlay) to a specific monitor by name, or cycles
// through monitors.
func BuildMoveMonitorCommand() *cobra.Command {
	var (
		monitorName string
		usePrevious bool
	)

	cmd := &cobra.Command{
		Use:   "move_monitor",
		Short: "Move cursor and overlay to another monitor",
		Long: `Move the cursor, and any active mode overlay (hints/grid/recursive-grid), to another monitor.

By default, cycles to the next monitor. Use --previous to cycle backwards.
Use --name to jump directly to a specific display by name.

Monitor names are matched case-insensitively against the localized display names
reported by macOS (e.g. "Built-in Retina Display", "DELL U2720Q").`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			hasName := cmd.Flags().Changed("name")

			if hasName && monitorName == "" {
				return derrors.New(
					derrors.CodeInvalidInput,
					"--name value must not be empty",
				)
			}

			if hasName && usePrevious {
				return derrors.New(
					derrors.CodeInvalidInput,
					"--previous cannot be used with --name",
				)
			}

			actionArgs := []string{"move_monitor"}

			if hasName {
				actionArgs = append(actionArgs, "--name="+monitorName)
			}

			if usePrevious {
				actionArgs = append(actionArgs, "--previous")
			}

			return sendCommand(cmd, "action", actionArgs)
		},
	}

	cmd.Flags().StringVar(&monitorName, "name", "",
		"Target monitor by display name (e.g. \"Built-in Retina Display\")")
	cmd.Flags().
		BoolVar(&usePrevious, "previous", false, "Cycle to the previous monitor instead of the next one")

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

// BuildFocusWindowCommand creates a focus_window cobra command that cycles
// window focus through all focusable windows on the active space.
func BuildFocusWindowCommand() *cobra.Command {
	var backward bool

	cmd := &cobra.Command{
		Use:   "focus_window",
		Short: "Cycle focus through windows on the active space",
		Long: `Cycle keyboard focus through all focusable windows on the current space.

Cycles forward through windows (or backward with --backward), wrapping at the
end. Only windows that are focusable (not minimized, not hidden) and on the
current space are included.`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			actionArgs := []string{"focus_window"}

			if backward {
				actionArgs = append(actionArgs, "--backward")
			}

			return sendCommand(cmd, "action", actionArgs)
		},
	}

	cmd.Flags().
		BoolVar(&backward, "backward", false, "Cycle to the previous window instead of the next one")

	return cmd
}

// BuildCycleHintCommand creates a cycle_hint cobra command that cycles through visible hints.
func BuildCycleHintCommand() *cobra.Command {
	var backward bool

	cmd := &cobra.Command{
		Use:   "cycle_hint",
		Short: "Cycle through visible hints",
		Long: `Cycle through visible hints in hints mode.

Cycles forward through hints (or backward with --backward), wrapping at the end.
Requires hints mode to be active.`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			actionArgs := []string{"cycle_hint"}

			if backward {
				actionArgs = append(actionArgs, "--backward")
			}

			return sendCommand(cmd, "action", actionArgs)
		},
	}

	cmd.Flags().
		BoolVar(&backward, "backward", false, "Cycle to the previous hint instead of the next one")

	return cmd
}

// BuildSpaceCommand creates a space cobra command that focuses a Mission
// Control space by its 1-based index using a synthetic dock swipe gesture
// since macOS exposes no public API to directly activate a space).
//
// Warning, fragile!!
func BuildSpaceCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "space <number>",
		Short: "Focus a Mission Control space by 1-based index",
		Long: `Focus a Mission Control space by its 1-based index.

Spaces are enumerated in Mission Control ordering across all connected
displays. Index 1 is the first space (typically the leftmost on the
primary display), index 2 the second, and so on.

macOS does not provide a public API to activate a space, so the daemon
synthesizes a high-velocity horizontal dock swipe gesture to fast-forward
to the destination space without the standard swipe animation. When the
destination sits on a different display, the cursor is warped to its
center first so the gesture is attributed to the correct screen.

Examples:
  neru action space 1     Focus the first Mission Control space
  neru action space 3     Focus the third`,
		Args: validateActionSpaceArgs,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return derrors.New(
					derrors.CodeInvalidInput,
					"space requires exactly one positional argument: the 1-based space number (e.g., neru action space 1)",
				)
			}

			actionArgs := []string{"space", strings.TrimSpace(args[0])}

			return sendCommand(cmd, "action", actionArgs)
		},
	}
}

func validateActionSpaceArgs(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return derrors.New(
			derrors.CodeInvalidInput,
			"space requires exactly one positional argument: the 1-based space number (e.g., neru action space 1)",
		)
	}

	raw := strings.TrimSpace(args[0])
	if raw == "" {
		return derrors.New(
			derrors.CodeInvalidInput,
			"space number cannot be empty",
		)
	}

	index, parseErr := strconv.Atoi(raw)
	if parseErr != nil || index < 1 {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"space number must be a positive integer, got %q",
			args[0],
		)
	}

	return nil
}
