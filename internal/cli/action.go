package cli

import (
	"github.com/spf13/cobra"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// ActionCmd is the CLI action command for performing immediate actions.
var ActionCmd = &cobra.Command{
	Use:   "action",
	Short: "Perform immediate mouse and scroll actions",
	Long:  `Perform immediate mouse and scroll actions at the current cursor position.`,
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
	"Perform left click at current cursor position",
	`Execute a left click at the current cursor location.`,
	[]string{"left_click"},
)

// ActionRightClickCmd is the right click action command.
var ActionRightClickCmd = BuildActionCommand(
	"right_click",
	"Perform right click at current cursor position",
	`Execute a right click at the current cursor location.`,
	[]string{"right_click"},
)

// ActionMouseUpCmd is the mouse up action command.
var ActionMouseUpCmd = BuildActionCommand(
	"mouse_up",
	"Release mouse button at current cursor position",
	`Release the left mouse button at the current cursor location.`,
	[]string{"mouse_up"},
)

// ActionMouseDownCmd is the mouse down action command.
var ActionMouseDownCmd = BuildActionCommand(
	"mouse_down",
	"Press mouse button at current cursor position",
	`Press and hold the left mouse button at the current cursor location.`,
	[]string{"mouse_down"},
)

// ActionMiddleClickCmd is the middle click action command.
var ActionMiddleClickCmd = BuildActionCommand(
	"middle_click",
	"Perform middle click at current cursor position",
	`Execute a middle click at the current cursor location.`,
	[]string{"middle_click"},
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
)

// ActionBackspaceCmd performs mode-aware backspace.
var ActionBackspaceCmd = BuildActionCommand(
	"backspace",
	"Apply backspace in current mode",
	`Apply mode-specific backspace behavior (hints input, grid input/subgrid, recursive-grid backtrack).`,
	[]string{"backspace"},
)

// ActionScrollUpCmd scrolls up at the current cursor position.
var ActionScrollUpCmd = BuildScrollActionCommand(
	"scroll_up",
	"Scroll up at current cursor position",
	`Scroll up at the current cursor location by scroll.scroll_step pixels.`,
)

// ActionScrollDownCmd scrolls down at the current cursor position.
var ActionScrollDownCmd = BuildScrollActionCommand(
	"scroll_down",
	"Scroll down at current cursor position",
	`Scroll down at the current cursor location by scroll.scroll_step pixels.`,
)

// ActionScrollLeftCmd scrolls left at the current cursor position.
var ActionScrollLeftCmd = BuildScrollActionCommand(
	"scroll_left",
	"Scroll left at current cursor position",
	`Scroll left at the current cursor location by scroll.scroll_step pixels.`,
)

// ActionScrollRightCmd scrolls right at the current cursor position.
var ActionScrollRightCmd = BuildScrollActionCommand(
	"scroll_right",
	"Scroll right at current cursor position",
	`Scroll right at the current cursor location by scroll.scroll_step pixels.`,
)

// ActionGoTopCmd scrolls to the top of the page.
var ActionGoTopCmd = BuildScrollActionCommand(
	"go_top",
	"Scroll to top of page",
	`Scroll to the top of the page at the current cursor location using scroll.scroll_step_full pixels.`,
)

// ActionGoBottomCmd scrolls to the bottom of the page.
var ActionGoBottomCmd = BuildScrollActionCommand(
	"go_bottom",
	"Scroll to bottom of page",
	`Scroll to the bottom of the page at the current cursor location using scroll.scroll_step_full pixels.`,
)

// ActionPageUpCmd scrolls up by half a page.
var ActionPageUpCmd = BuildScrollActionCommand(
	"page_up",
	"Scroll up by half page",
	`Scroll up by half a page at the current cursor location using scroll.scroll_step_half pixels.`,
)

// ActionPageDownCmd scrolls down by half a page.
var ActionPageDownCmd = BuildScrollActionCommand(
	"page_down",
	"Scroll down by half page",
	`Scroll down by half a page at the current cursor location using scroll.scroll_step_half pixels.`,
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
