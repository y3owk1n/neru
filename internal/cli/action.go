package cli

import (
	"github.com/spf13/cobra"
)

var actionCmd = &cobra.Command{
	Use:   "action",
	Short: "Perform actions at the current cursor position",
	Long:  `Execute mouse actions immediately at the current cursor location without target selection.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

var actionLeftClickCmd = &cobra.Command{
	Use:   "left_click",
	Short: "Perform left click at current cursor position",
	Long:  `Execute a left click at the current cursor location.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		var params []string
		params = append(params, "left_click")

		return sendCommand(cmd, "action", params)
	},
}

var actionRightClickCmd = &cobra.Command{
	Use:   "right_click",
	Short: "Perform right click at current cursor position",
	Long:  `Execute a right click at the current cursor location.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		var params []string
		params = append(params, "right_click")

		return sendCommand(cmd, "action", params)
	},
}

var actionMouseUpCmd = &cobra.Command{
	Use:   "mouse_up",
	Short: "Release mouse button at current cursor position",
	Long:  `Release the left mouse button at the current cursor location.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		var params []string
		params = append(params, "mouse_up")

		return sendCommand(cmd, "action", params)
	},
}

var actionMouseDownCmd = &cobra.Command{
	Use:   "mouse_down",
	Short: "Press mouse button at current cursor position",
	Long:  `Press and hold the left mouse button at the current cursor location.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		var params []string
		params = append(params, "mouse_down")

		return sendCommand(cmd, "action", params)
	},
}

var actionMiddleClickCmd = &cobra.Command{
	Use:   "middle_click",
	Short: "Perform middle click at current cursor position",
	Long:  `Execute a middle click at the current cursor location.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		var params []string
		params = append(params, "middle_click")

		return sendCommand(cmd, "action", params)
	},
}

var actionScrollCmd = &cobra.Command{
	Use:   "scroll",
	Short: "Enter scroll mode at current cursor position",
	Long:  `Activate scroll mode at the current cursor location.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		var params []string
		params = append(params, "scroll")

		return sendCommand(cmd, "action", params)
	},
}

func init() {
	actionCmd.AddCommand(actionLeftClickCmd)
	actionCmd.AddCommand(actionRightClickCmd)
	actionCmd.AddCommand(actionMouseUpCmd)
	actionCmd.AddCommand(actionMouseDownCmd)
	actionCmd.AddCommand(actionMiddleClickCmd)
	actionCmd.AddCommand(actionScrollCmd)

	rootCmd.AddCommand(actionCmd)
}
