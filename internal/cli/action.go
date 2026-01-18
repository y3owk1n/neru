package cli

import (
	"github.com/spf13/cobra"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// ActionCmd is the CLI action command for performing immediate actions.
var ActionCmd = &cobra.Command{
	Use:   "action",
	Short: "Perform immediate mouse actions",
	Long:  `Perform immediate mouse actions at the current cursor position.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return derrors.New(
			derrors.CodeInvalidInput,
			"action subcommand required (e.g., neru action left_click)",
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

func init() {
	ActionCmd.AddCommand(ActionLeftClickCmd)
	ActionCmd.AddCommand(ActionRightClickCmd)
	ActionCmd.AddCommand(ActionMouseUpCmd)
	ActionCmd.AddCommand(ActionMouseDownCmd)
	ActionCmd.AddCommand(ActionMiddleClickCmd)
	ActionCmd.AddCommand(ActionMoveMouseCmd)
	ActionCmd.AddCommand(ActionMoveMouseRelativeCmd)

	RootCmd.AddCommand(ActionCmd)
}
