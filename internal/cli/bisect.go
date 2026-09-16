package cli

import "github.com/y3owk1n/neru/internal/domain"

// BisectCmd is the CLI bisect command.
var BisectCmd = BuildModeCommand(ModeConfig{
	Mode:  domain.ModeBisect,
	Short: "Activate bisect navigation mode",
	Long: `Bisect mode narrows a region by halves until the cursor is on the target.

The region starts as the whole screen, or the focused window with
--capture-scope window, and is drawn divided in four. Each press keeps one
half of it, or one quadrant, and the cursor moves to the center of what is
left. Each press needs one judgement, which side of the center the target is
on. Ten quadrant cuts take a 1920 by 1080 screen down to two pixels.

Default keys (vim-style, plus the arrow keys):
  h j k l          keep the left, bottom, top, right half
  y u b n          keep the top-left, top-right, bottom-left, bottom-right quadrant
  Backtick         toggle whether the real cursor follows the center
  Space            start over from the whole region
  Backspace        take the last cut back
  Shift+L / R / M  left, right, middle click
  Escape           exit

Examples:
  neru bisect                          # Start over the whole screen
  neru bisect --capture-scope window   # Start over the focused window
  neru bisect --cursor-selection-mode hold   # Keep the cursor still until a click
  neru bisect --toggle                 # Enter, or leave if already in it`,
})

func init() {
	RootCmd.AddCommand(BisectCmd)
}
