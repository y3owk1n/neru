package cli

import "github.com/y3owk1n/neru/internal/domain"

// ScrollCmd is the CLI scroll command.
var ScrollCmd = BuildModeCommand(ModeConfig{
	Mode:  domain.ModeScroll,
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
})

func init() {
	RootCmd.AddCommand(ScrollCmd)
}
