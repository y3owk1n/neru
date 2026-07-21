package cli

import (
	"strings"

	"github.com/spf13/cobra"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// ActionCmd is the CLI action command for performing immediate actions.
var ActionCmd = &cobra.Command{
	Use:   "action",
	Short: "Perform immediate mouse, scroll, and keyboard actions",
	Long: `Perform immediate mouse, scroll, and keyboard actions without entering a mode.

Point-targeted actions use the active mode selection when one exists. Use
--bare to force current-cursor targeting. Most click and scroll actions
support --modifier to hold modifier keys during the action.

Available subcommands:
  Click actions:    left_click, right_click, middle_click, mouse_down, mouse_up
  Scroll actions:   scroll_up, scroll_down, scroll_left, scroll_right,
                    go_top, go_bottom, page_up, page_down
  Mouse movement:   move_mouse, move_mouse_relative, move_monitor
  Mode control:     reset, backspace, wait_for_mode_exit, cycle_hint
   Cursor saving:    save_cursor_pos, restore_cursor_pos
   Cursor visibility: hide_cursor, show_cursor
   Key injection:    feed

Click actions can be chained with commas to produce multi-click sequences:
  neru action left_click,left_click              Double-click at cursor
  neru action left_click,left_click,left_click    Triple-click at cursor

Examples:
  neru action left_click                        Click at current cursor
  neru action left_click,left_click             Double-click at cursor
  neru action scroll_down --steps 5             Scroll down 5 steps
  neru action move_mouse --x 1920 --y 1080      Move to absolute position
  neru action feed ctrl+c                        Send Ctrl+C keystroke`,
	DisableFlagParsing: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 && strings.Contains(args[0], ",") {
			// Comma-separated action chain (e.g. "left_click,left_click")
			// doesn't match a subcommand name. Forward it directly through IPC
			// where handleActionChain will split and execute each action.
			return sendCommand(cmd, "action", args)
		}

		return derrors.New(
			derrors.CodeInvalidInput,
			"action subcommand required (e.g., neru action left_click, neru action scroll_up)",
		)
	},
}

// ActionLeftClickCmd is the left click action command.
var ActionLeftClickCmd = BuildActionCommand(
	"left_click",
	"Perform left click",
	`Execute a left click.

Targets the active mode selection when one exists, otherwise clicks at
the current cursor location. Use --modifier to hold modifier keys
(e.g. --modifier shift for Shift+click).`,
	[]string{"left_click"},
	true,
)

// ActionRightClickCmd is the right click action command.
var ActionRightClickCmd = BuildActionCommand(
	"right_click",
	"Perform right click",
	`Execute a right click.

Targets the active mode selection when one exists, otherwise clicks at
the current cursor location. Use --modifier to hold modifier keys
(e.g. --modifier option for Option+click).`,
	[]string{"right_click"},
	true,
)

// ActionMouseUpCmd is the mouse up action command.
var ActionMouseUpCmd = BuildActionCommand(
	"mouse_up",
	"Release mouse button",
	`Release the left mouse button.

Useful for drag-and-drop workflows with mouse_down. Targets the active
mode selection when one exists, otherwise the current cursor location.`,
	[]string{"mouse_up"},
	true,
)

// ActionMouseDownCmd is the mouse down action command.
var ActionMouseDownCmd = BuildActionCommand(
	"mouse_down",
	"Press mouse button",
	`Press and hold the left mouse button.

Intended for drag operations: use mouse_down at the start point, move
the cursor, then mouse_up at the destination. Targets the active mode
selection when one exists, otherwise the current cursor location.`,
	[]string{"mouse_down"},
	true,
)

// ActionMiddleClickCmd is the middle click action command.
var ActionMiddleClickCmd = BuildActionCommand(
	"middle_click",
	"Perform middle click",
	`Execute a middle click (useful for opening links in new tabs).

Targets the active mode selection when one exists, otherwise clicks at
the current cursor location. Use --modifier to hold modifier keys.`,
	[]string{"middle_click"},
	true,
)

// ActionMoveMouseCmd is the move mouse action command.
var ActionMoveMouseCmd = BuildMoveMouseCommand()

// ActionMoveMouseRelativeCmd is the move mouse relative action command.
var ActionMoveMouseRelativeCmd = BuildMoveMouseRelativeCommand()

// ActionMoveMonitorCmd is the move monitor action command.
var ActionMoveMonitorCmd = BuildMoveMonitorCommand()

// ActionResetCmd resets current mode state.
var ActionResetCmd = BuildActionCommand(
	"reset",
	"Reset current mode input state",
	`Reset the active mode state (grid input, recursive-grid depth, etc.) without exiting.`,
	[]string{"reset"},
	false,
)

// ActionBackspaceCmd performs mode-aware backspace.
var ActionBackspaceCmd = BuildActionCommand(
	"backspace",
	"Apply backspace in current mode",
	`Apply mode-specific backspace behavior (hints input, grid input/subgrid, recursive-grid backtrack).`,
	[]string{"backspace"},
	false,
)

// ActionWaitForModeExitCmd blocks until the current mode exits.
// Has its own builder to support the --bail flag.
var ActionWaitForModeExitCmd = func() *cobra.Command {
	var bail bool

	cmd := &cobra.Command{
		Use:   "wait_for_mode_exit",
		Short: "Wait until mode exits",
		Long: `Block until the current mode exits and Neru returns to idle.

Use --bail in an action chain to abort the chain when the mode exits
without a completed selection (e.g. user presses Escape).`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			args := []string{"wait_for_mode_exit"}
			if bail {
				args = append(args, "--bail")
			}

			return sendCommand(cmd, "action", args)
		},
	}

	cmd.Flags().BoolVar(&bail, "bail", false,
		"Abort the action chain if the mode exits without a selection")

	return cmd
}()

// ActionSaveCursorPosCmd saves cursor position for later restoration.
var ActionSaveCursorPosCmd = BuildActionCommand(
	"save_cursor_pos",
	"Save current cursor position",
	`Save the current cursor position so it can be restored later with restore_cursor_pos.`,
	[]string{"save_cursor_pos"},
	false,
)

// ActionRestoreCursorPosCmd restores previously saved cursor position.
var ActionRestoreCursorPosCmd = BuildActionCommand(
	"restore_cursor_pos",
	"Restore saved cursor position",
	`Restore cursor position previously saved by save_cursor_pos.`,
	[]string{"restore_cursor_pos"},
	false,
)

// ActionHideCursorCmd hides the system cursor.
var ActionHideCursorCmd = BuildActionCommand(
	"hide_cursor",
	"Hide system cursor",
	`Hide the system cursor. Used together with show_cursor to create a virtual
pointer experience. The cursor stays hidden until show_cursor is called or
the application exits.`,
	[]string{"hide_cursor"},
	false,
)

// ActionShowCursorCmd shows the system cursor.
var ActionShowCursorCmd = BuildActionCommand(
	"show_cursor",
	"Show system cursor",
	`Show the system cursor after it was hidden by hide_cursor.`,
	[]string{"show_cursor"},
	false,
)

// ActionScrollUpCmd scrolls up at the current cursor position.
var ActionScrollUpCmd = BuildScrollActionCommand(
	"scroll_up",
	"Scroll up",
	`Scroll up by a configurable step amount.

Use --steps to override the step size (in pixels). Targets the active
mode selection when one exists, otherwise scrolls at the cursor location.`,
	true,
)

// ActionScrollDownCmd scrolls down at the current cursor position.
var ActionScrollDownCmd = BuildScrollActionCommand(
	"scroll_down",
	"Scroll down",
	`Scroll down by a configurable step amount.

Use --steps to override the step size (in pixels). Targets the active
mode selection when one exists, otherwise scrolls at the cursor location.`,
	true,
)

// ActionScrollLeftCmd scrolls left at the current cursor position.
var ActionScrollLeftCmd = BuildScrollActionCommand(
	"scroll_left",
	"Scroll left",
	`Scroll left by a configurable step amount.

Use --steps to override the step size (in pixels). Targets the active
mode selection when one exists, otherwise scrolls at the cursor location.`,
	true,
)

// ActionScrollRightCmd scrolls right at the current cursor position.
var ActionScrollRightCmd = BuildScrollActionCommand(
	"scroll_right",
	"Scroll right",
	`Scroll right by a configurable step amount.

Use --steps to override the step size (in pixels). Targets the active
mode selection when one exists, otherwise scrolls at the cursor location.`,
	true,
)

// ActionGoTopCmd scrolls to the top of the page.
var ActionGoTopCmd = BuildScrollActionCommand(
	"go_top",
	"Scroll to top of page",
	`Scroll to the top of the page.

Targets the active mode selection when one exists, otherwise scrolls
at the current cursor location.`,
	false,
)

// ActionGoBottomCmd scrolls to the bottom of the page.
var ActionGoBottomCmd = BuildScrollActionCommand(
	"go_bottom",
	"Scroll to bottom of page",
	`Scroll to the bottom of the page.

Targets the active mode selection when one exists, otherwise scrolls
at the current cursor location.`,
	false,
)

// ActionPageUpCmd scrolls up by half a page.
var ActionPageUpCmd = BuildScrollActionCommand(
	"page_up",
	"Scroll up by half page",
	`Scroll up by approximately half the visible page height.

Targets the active mode selection when one exists, otherwise scrolls
at the current cursor location.`,
	false,
)

// ActionPageDownCmd scrolls down by half a page.
var ActionPageDownCmd = BuildScrollActionCommand(
	"page_down",
	"Scroll down by half page",
	`Scroll down by approximately half the visible page height.

Targets the active mode selection when one exists, otherwise scrolls
at the current cursor location.`,
	false,
)

// ActionCycleHintCmd cycles through visible hints in hints mode.
var ActionCycleHintCmd = BuildCycleHintCommand()

func init() {
	ActionCmd.AddCommand(ActionLeftClickCmd)
	ActionCmd.AddCommand(ActionRightClickCmd)
	ActionCmd.AddCommand(ActionMouseUpCmd)
	ActionCmd.AddCommand(ActionMouseDownCmd)
	ActionCmd.AddCommand(ActionMiddleClickCmd)
	ActionCmd.AddCommand(ActionMoveMouseCmd)
	ActionCmd.AddCommand(ActionMoveMouseRelativeCmd)
	ActionCmd.AddCommand(ActionMoveMonitorCmd)
	ActionCmd.AddCommand(ActionFeedCmd)
	ActionCmd.AddCommand(ActionResetCmd)
	ActionCmd.AddCommand(ActionBackspaceCmd)
	ActionCmd.AddCommand(ActionWaitForModeExitCmd)
	ActionCmd.AddCommand(ActionSaveCursorPosCmd)
	ActionCmd.AddCommand(ActionRestoreCursorPosCmd)
	ActionCmd.AddCommand(ActionScrollUpCmd)
	ActionCmd.AddCommand(ActionScrollDownCmd)
	ActionCmd.AddCommand(ActionScrollLeftCmd)
	ActionCmd.AddCommand(ActionScrollRightCmd)
	ActionCmd.AddCommand(ActionGoTopCmd)
	ActionCmd.AddCommand(ActionGoBottomCmd)
	ActionCmd.AddCommand(ActionPageUpCmd)
	ActionCmd.AddCommand(ActionPageDownCmd)
	ActionCmd.AddCommand(ActionCycleHintCmd)
	ActionCmd.AddCommand(ActionHideCursorCmd)
	ActionCmd.AddCommand(ActionShowCursorCmd)

	RootCmd.AddCommand(ActionCmd)
}
