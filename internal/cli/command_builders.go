// Command factories shared by the generated CLI surface: each Build*Command
// returns a cobra command that forwards its action to the running daemon over
// IPC. root.go owns process bootstrap; the factories live here.

package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
)

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

// BuildToggleCommand creates a cobra command for a runtime toggle, adding the
// --state flag that asks for a state instead of flipping whatever is there.
//
// Flipping is right for a key binding, where the user sees the result and
// presses again if it went the wrong way. A script has no such feedback loop,
// so it needs to be able to name the state it wants — and to read it back from
// "neru status --json", which reports every toggle these commands change.
func BuildToggleCommand(use, short, long string, action string) *cobra.Command {
	var state string

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.NoArgs,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if state == "" {
				return sendCommand(cmd, action, nil)
			}

			if state != "on" && state != "off" && state != "toggle" {
				return derrors.Newf(
					derrors.CodeInvalidInput,
					"invalid --state %q: expected on, off, or toggle",
					state,
				)
			}

			return sendCommand(cmd, action, []string{"--state=" + state})
		},
	}

	cmd.Flags().StringVar(&state, "state", "",
		"State to converge on: on, off, or toggle (default: toggle)")

	return cmd
}

// BuildActionCommand creates an action cobra command with the given parameters.
func BuildActionCommand(
	use, short, long string,
	params []string,
	allowTargetOverride bool,
) *cobra.Command {
	return buildActionCommand(use, short, long, params, allowTargetOverride, false)
}

// BuildClickActionCommand creates an action cobra command for a mouse button,
// adding the --state and --toggle flags that select which half of the click to
// perform.
func BuildClickActionCommand(use, short, long string, params []string) *cobra.Command {
	return buildActionCommand(use, short, long, params, true, true)
}

// buildActionCommand creates an action cobra command. allowButtonPhase adds the
// --state and --toggle flags, which only mouse button actions accept.
func buildActionCommand(
	use, short, long string,
	params []string,
	allowTargetOverride bool,
	allowButtonPhase bool,
) *cobra.Command {
	var (
		modifier  string
		selection bool
		bare      bool
		state     string
		toggle    bool
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

			if state != "" && toggle {
				return derrors.New(
					derrors.CodeInvalidInput,
					"--state and --toggle cannot be used together",
				)
			}

			if state != "" && state != "down" && state != "up" {
				return derrors.Newf(
					derrors.CodeInvalidInput,
					"invalid --state %q: expected down or up",
					state,
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

			if state != "" {
				args = append(args, "--state="+state)
			}

			if toggle {
				args = append(args, "--toggle")
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

	if allowButtonPhase {
		cmd.Flags().StringVar(
			&state,
			"state",
			"",
			"Perform only one half of the click: down presses and holds, up releases",
		)
		cmd.Flags().BoolVar(
			&toggle,
			"toggle",
			false,
			"Release the button when it is held, press and hold it otherwise",
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
		modifier  string
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
			if modifier != "" {
				args = append(args, "--modifier="+modifier)
			}

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

	cmd.Flags().StringVar(&modifier, "modifier", "",
		"Comma-separated modifier keys to hold during the scroll (cmd, super, meta, shift, alt, option, ctrl)")
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

// BuildMoveCellCommand creates a move_cell cobra command that slides the
// active mode's selection to a neighboring cell on the same layer.
func BuildMoveCellCommand() *cobra.Command {
	var (
		direction string
		count     int
	)

	cmd := &cobra.Command{
		Use:   "move_cell",
		Short: "Move the current selection to a neighboring cell",
		Long: `Slide the active mode's selection one cell in a direction, without
changing the active layer.

In recursive-grid mode the highlighted region moves at the current depth,
crossing into a neighboring parent region when it reaches the edge of its
own. In grid mode an open subgrid moves to the neighboring cell.

Movement stops at the screen edge instead of wrapping, and does nothing in
modes that have no cell selection.

Examples:
  neru action move_cell --direction right
  neru action move_cell --direction up --count 3`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, dirErr := domain.ParseDirection(direction)
			if dirErr != nil {
				return dirErr
			}

			if count < 1 {
				return derrors.New(
					derrors.CodeInvalidInput,
					"--count must be at least 1",
				)
			}

			actionArgs := []string{
				"move_cell",
				"--direction=" + direction,
				fmt.Sprintf("--count=%d", count),
			}

			return sendCommand(cmd, "action", actionArgs)
		},
	}

	cmd.Flags().StringVar(&direction, "direction", "", "Direction to move (left, right, up, down)")
	cmd.Flags().IntVar(&count, "count", 1, "Number of cells to move")
	_ = cmd.MarkFlagRequired("direction")

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
