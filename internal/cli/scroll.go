package cli

import (
	"github.com/spf13/cobra"
)

// ScrollCmd is the CLI scroll command.
var ScrollCmd = &cobra.Command{
	Use:   "scroll",
	Short: "Launch scroll mode for vim-style scrolling",
	Long: `Activate scroll mode for keyboard-driven scrolling at the cursor position.

Once in scroll mode, use vim-style keys to scroll:
  j / k     Scroll down / up
  h / l     Scroll left / right
  d / u     Page down / page up
  gg / G    Top / bottom of page

Press Escape to exit scroll mode and return to idle.

Examples:
  neru scroll           Activate scroll mode at the current cursor position
  neru scroll --toggle  Toggle scroll mode on/off`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		toggleFlag, err := cmd.Flags().GetBool("toggle")
		if err != nil {
			return err
		}

		params := []string{}
		if toggleFlag {
			params = append(params, "--toggle")
		}

		return sendCommand(cmd, "scroll", params)
	},
}

func init() {
	ScrollCmd.Flags().BoolP(
		"toggle", "t", false,
		"Toggle scroll mode on/off (exit to idle if already active)",
	)

	RootCmd.AddCommand(ScrollCmd)
}

func init() {
	RootCmd.AddCommand(ScrollCmd)
}
