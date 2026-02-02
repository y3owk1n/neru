package cli

// QuadGridCmd is the CLI quadgrid command.
var QuadGridCmd = BuildModeCommand(ModeConfig{
	Name:  "quadgrid",
	Short: "Activate quad-grid navigation mode",
	Long: `Quad-grid mode provides recursive quadrant-based navigation.

The screen is divided into 4 quadrants (default keys: u,i,j,k).
Each selection recursively narrows the active area until minimum size is reached.

Key mappings (warpd convention):
  u = upper-left quadrant    i = upper-right quadrant
  j = lower-left quadrant    k = lower-right quadrant

Navigation:
  - Press quadrant key to narrow selection
  - Press backspace to backtrack
  - Press reset key (default: comma) to start over
  - Press exit key (default: escape) to exit mode

Examples:
  neru quadgrid                    # Start quad-grid mode
  neru quadgrid --action click     # Start with pending click action`,
	ActionDesc: "quad-grid selection",
})

func init() {
	RootCmd.AddCommand(QuadGridCmd)
}
