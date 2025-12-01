package cli

import (
	"github.com/spf13/cobra"
)

// ActionCmd is the CLI action command.
var ActionCmd = &cobra.Command{
	Use:   "action",
	Short: "Enter action mode or perform immediate actions",
	Long:  `Enter interactive action mode to perform mouse actions, or use subcommands for immediate actions.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		// No subcommand provided, enter action mode
		return sendCommand(cmd, "action", []string{})
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

func init() {
	ActionCmd.AddCommand(ActionLeftClickCmd)
	ActionCmd.AddCommand(ActionRightClickCmd)
	ActionCmd.AddCommand(ActionMouseUpCmd)
	ActionCmd.AddCommand(ActionMouseDownCmd)
	ActionCmd.AddCommand(ActionMiddleClickCmd)

	RootCmd.AddCommand(ActionCmd)
}
