// Package recursivegrid provides recursive cell-based navigation for screen coordinates.
//
// The recursivegrid system divides the screen into a grid with configurable columns and rows,
// allowing users to navigate by repeatedly selecting cells until reaching a minimum size threshold.
//
// Key Features:
//   - Recursive division: Each selection narrows the active area
//   - Configurable dimensions: Supports both square (NxN) and non-square (CxR) grids
//   - Configurable limits: Minimum size and maximum depth constraints
//   - Backtracking: Support for undoing selections via backspace
//   - warpd-compatible: Uses u/i/j/k key mapping by default (top-left, top-right, bottom-left, bottom-right)
//
// Basic Usage:
//
//	// Create a new recursive grid for a 1920x1080 screen
//	bounds := image.Rect(0, 0, 1920, 1080)
//	grid := recursivegrid.NewRecursiveGrid(bounds, 25, 10) // 25px min, 10 max depth
//
//	// Select top-left cell (key 'u')
//	center, complete := grid.SelectCell(recursivegrid.TopLeft)
//
//	// center is the cursor position to move to
//	// complete is true if minimum size has been reached
//
// Manager Usage:
//
//	// Create a manager with callbacks
//	manager := recursivegrid.NewManager(
//	    bounds,
//	    "uijk",                    // Key mapping
//	    ",",                       // Reset key
//	    []string{"escape"},        // Exit keys
//	    func() { /* update overlay */ },
//	    func(point) { /* selection complete */ },
//	    logger,
//	)
//
//	// Process key input
//	point, completed, shouldExit := manager.HandleInput("u")
//
// Key Mapping:
//   - Default: u (top-left), i (top-right), j (bottom-left), k (bottom-right)
//   - Customizable via N-character string (where N = grid_cols * grid_rows)
//
// Exit Conditions:
//   - Cell size < minimum size (default 25px)
//   - Maximum recursion depth reached (default 10)
//   - User presses exit key
//
// The package is designed to integrate with the Neru mode system and follows
// the same patterns as the existing grid and hints modes.
package recursivegrid
