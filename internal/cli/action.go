package cli

import (
	"github.com/spf13/cobra"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// ActionCmd is the CLI action command for performing immediate actions.
var ActionCmd = &cobra.Command{
	Use:   "action",
	Short: "Perform immediate mouse and scroll actions",
	Long: `Perform immediate mouse and scroll actions.

Point-targeted actions use the active mode selection when one exists. Use
--bare to force current-cursor targeting.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
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
	`Execute a left click at the active selection when available, otherwise at the current cursor location.`,
	[]string{"left_click"},
	true,
)

// ActionRightClickCmd is the right click action command.
var ActionRightClickCmd = BuildActionCommand(
	"right_click",
	"Perform right click",
	`Execute a right click at the active selection when available, otherwise at the current cursor location.`,
	[]string{"right_click"},
	true,
)

// ActionMouseUpCmd is the mouse up action command.
var ActionMouseUpCmd = BuildActionCommand(
	"mouse_up",
	"Release mouse button",
	`Release the left mouse button at the active selection when available, otherwise at the current cursor location.`,
	[]string{"mouse_up"},
	true,
)

// ActionMouseDownCmd is the mouse down action command.
var ActionMouseDownCmd = BuildActionCommand(
	"mouse_down",
	"Press mouse button",
	`Press and hold the left mouse button at the active selection when available, otherwise at the current cursor location.`,
	[]string{"mouse_down"},
	true,
)

// ActionMiddleClickCmd is the middle click action command.
var ActionMiddleClickCmd = BuildActionCommand(
	"middle_click",
	"Perform middle click",
	`Execute a middle click at the active selection when available, otherwise at the current cursor location.`,
	[]string{"middle_click"},
	true,
)

// ActionMoveMouseCmd is the move mouse action command.
var ActionMoveMouseCmd = BuildMoveMouseCommand()

// ActionMoveMouseRelativeCmd is the move mouse relative action command.
var ActionMoveMouseRelativeCmd = BuildMoveMouseRelativeCommand()

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
var ActionWaitForModeExitCmd = BuildActionCommand(
	"wait_for_mode_exit",
	"Wait until mode exits",
	`Block until the current mode exits and Neru returns to idle.`,
	[]string{"wait_for_mode_exit"},
	false,
)

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

// ActionScrollUpCmd scrolls up at the current cursor position.
var ActionScrollUpCmd = BuildScrollActionCommand(
	"scroll_up",
	"Scroll up",
	`Scroll up at the active selection when available, otherwise at the current cursor location.`,
)

// ActionScrollDownCmd scrolls down at the current cursor position.
var ActionScrollDownCmd = BuildScrollActionCommand(
	"scroll_down",
	"Scroll down",
	`Scroll down at the active selection when available, otherwise at the current cursor location.`,
)

// ActionScrollLeftCmd scrolls left at the current cursor position.
var ActionScrollLeftCmd = BuildScrollActionCommand(
	"scroll_left",
	"Scroll left",
	`Scroll left at the active selection when available, otherwise at the current cursor location.`,
)

// ActionScrollRightCmd scrolls right at the current cursor position.
var ActionScrollRightCmd = BuildScrollActionCommand(
	"scroll_right",
	"Scroll right",
	`Scroll right at the active selection when available, otherwise at the current cursor location.`,
)

// ActionGoTopCmd scrolls to the top of the page.
var ActionGoTopCmd = BuildScrollActionCommand(
	"go_top",
	"Scroll to top of page",
	`Scroll to the top of the page at the active selection when available, otherwise at the current cursor location.`,
)

// ActionGoBottomCmd scrolls to the bottom of the page.
var ActionGoBottomCmd = BuildScrollActionCommand(
	"go_bottom",
	"Scroll to bottom of page",
	`Scroll to the bottom of the page at the active selection when available, otherwise at the current cursor location.`,
)

// ActionPageUpCmd scrolls up by half a page.
var ActionPageUpCmd = BuildScrollActionCommand(
	"page_up",
	"Scroll up by half page",
	`Scroll up by half a page at the active selection when available, otherwise at the current cursor location.`,
)

// ActionPageDownCmd scrolls down by half a page.
var ActionPageDownCmd = BuildScrollActionCommand(
	"page_down",
	"Scroll down by half page",
	`Scroll down by half a page at the active selection when available, otherwise at the current cursor location.`,
)

func init() {
	ActionCmd.AddCommand(ActionLeftClickCmd)
	ActionCmd.AddCommand(ActionRightClickCmd)
	ActionCmd.AddCommand(ActionMouseUpCmd)
	ActionCmd.AddCommand(ActionMouseDownCmd)
	ActionCmd.AddCommand(ActionMiddleClickCmd)
	ActionCmd.AddCommand(ActionMoveMouseCmd)
	ActionCmd.AddCommand(ActionMoveMouseRelativeCmd)
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

	RootCmd.AddCommand(ActionCmd)
}
