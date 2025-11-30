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

var actionLeftClickCmd = buildActionCommand(
	"left_click",
	"Perform left click at current cursor position",
	`Execute a left click at the current cursor location.`,
	[]string{"left_click"},
)

var actionRightClickCmd = buildActionCommand(
	"right_click",
	"Perform right click at current cursor position",
	`Execute a right click at the current cursor location.`,
	[]string{"right_click"},
)

var actionMouseUpCmd = buildActionCommand(
	"mouse_up",
	"Release mouse button at current cursor position",
	`Release the left mouse button at the current cursor location.`,
	[]string{"mouse_up"},
)

var actionMouseDownCmd = buildActionCommand(
	"mouse_down",
	"Press mouse button at current cursor position",
	`Press and hold the left mouse button at the current cursor location.`,
	[]string{"mouse_down"},
)

var actionMiddleClickCmd = buildActionCommand(
	"middle_click",
	"Perform middle click at current cursor position",
	`Execute a middle click at the current cursor location.`,
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
