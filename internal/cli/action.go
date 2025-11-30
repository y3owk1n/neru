package cli

import (
	"github.com/spf13/cobra"
)

var actionCmd = &cobra.Command{
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

var actionLeftClickCmd = builder.BuildActionCommand(
	"left_click",
	"Perform left click at current cursor position",
	`Execute a left click at the current cursor location.`,
	"action",
	[]string{"left_click"},
)

var actionRightClickCmd = builder.BuildActionCommand(
	"right_click",
	"Perform right click at current cursor position",
	`Execute a right click at the current cursor location.`,
	"action",
	[]string{"right_click"},
)

var actionMouseUpCmd = builder.BuildActionCommand(
	"mouse_up",
	"Release mouse button at current cursor position",
	`Release the left mouse button at the current cursor location.`,
	"action",
	[]string{"mouse_up"},
)

var actionMouseDownCmd = builder.BuildActionCommand(
	"mouse_down",
	"Press mouse button at current cursor position",
	`Press and hold the left mouse button at the current cursor location.`,
	"action",
	[]string{"mouse_down"},
)

var actionMiddleClickCmd = builder.BuildActionCommand(
	"middle_click",
	"Perform middle click at current cursor position",
	`Execute a middle click at the current cursor location.`,
	"action",
	[]string{"middle_click"},
)

func init() {
	actionCmd.AddCommand(actionLeftClickCmd)
	actionCmd.AddCommand(actionRightClickCmd)
	actionCmd.AddCommand(actionMouseUpCmd)
	actionCmd.AddCommand(actionMouseDownCmd)
	actionCmd.AddCommand(actionMiddleClickCmd)

	rootCmd.AddCommand(actionCmd)
}
