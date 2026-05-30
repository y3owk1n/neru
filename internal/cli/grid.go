package cli

// GridCmd is the CLI grid command.
var GridCmd = BuildModeCommand(ModeConfig{
	Name:  "grid",
	Short: "Launch grid mode for mouseless navigation",
	Long: `Overlay a grid on the screen and navigate to any point by subdivision.

Grid mode divides the screen into a coarse grid. Type the grid coordinates
to zoom into a region, then keep narrowing down until you reach the
desired point. A click, right-click, or other action is then performed.

This is useful for reaching UI elements that hints mode cannot detect
(e.g., custom-rendered canvases, graphics, or embedded web views).

Use --action to set a pending action once the grid target is selected.

Examples:
  neru grid                           Activate grid mode (navigate to a point)
  neru grid --action left_click       Navigate and then left-click
  neru grid --action right_click      Navigate and then right-click
  neru grid --action left_click --repeat  Click multiple spots in sequence`,
	ActionDesc: "grid selection",
})

func init() {
	RootCmd.AddCommand(GridCmd)
}
