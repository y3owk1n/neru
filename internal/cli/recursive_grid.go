package cli

// RecursiveGridCmd is the CLI recursive_grid command.
var RecursiveGridCmd = BuildModeCommand(ModeConfig{
	Name:  "recursive_grid",
	Short: "Activate recursive-grid navigation mode",
	Long: `Recursive-grid mode provides recursive cell-based navigation.

The screen is divided into NxN cells (default 2x2, keys: u,i,j,k).
Each selection recursively narrows the active area until minimum size is reached.

Key mappings (warpd convention):
  u = upper-left cell    i = upper-right cell
  j = lower-left cell    k = lower-right cell

Navigation:
  - Press cell key to narrow selection
  - Press backspace to backtrack
  - Press reset key (default: comma) to start over
  - Press exit key (default: escape) to exit mode

Examples:
  neru recursive_grid                    # Start recursive-grid mode
  neru recursive_grid --action click     # Start with pending click action`,
	ActionDesc: "recursive-grid selection",
})

func init() {
	RootCmd.AddCommand(RecursiveGridCmd)
}
