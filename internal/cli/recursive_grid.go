package cli

import "github.com/y3owk1n/neru/internal/domain"

// RecursiveGridCmd is the CLI recursive_grid command.
var RecursiveGridCmd = BuildModeCommand(ModeConfig{
	Mode:    domain.ModeRecursiveGrid,
	Aliases: []string{"recursive-grid"},
	Short:   "Activate recursive-grid navigation mode",
	Long: `Recursive-grid mode provides recursive cell-based navigation.

The screen is divided into a grid of cells (default 3x3, keys: r,t,y,f,g,h,v,b,n).
Each selection recursively narrows the active area until minimum size is reached.

Key mappings (default 3x3 grid, left to right then top to bottom):
  r = upper-left    t = upper-middle    y = upper-right
  f = middle-left   g = center          h = middle-right
  v = lower-left    b = lower-middle    n = lower-right

Navigation:
  - Press cell key to narrow selection
  - Press backspace to backtrack
  - Bind "action move_cell --direction=<dir>" to slide the selection to a
    neighboring cell at the current depth
  - Press reset key (default: comma) to start over
  - Press exit key (default: escape) to exit mode

Examples:
  neru recursive_grid                                  # Start recursive-grid mode
  neru recursive_grid --action click                   # Start with pending click action
  neru recursive_grid --action left_click --repeat     # Click and restart mode
  neru recursive_grid --zoom-to-depth 2                # Zoom to depth 2 at cursor
  neru recursive_grid --zoom-to-depth 3 --action click # Zoom to depth 3 and click`,
})

func init() {
	RootCmd.AddCommand(RecursiveGridCmd)
}
