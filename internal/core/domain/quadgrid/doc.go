// Package quadgrid provides recursive quadrant-based navigation for screen coordinates.
//
// The quadgrid system divides the screen into 4 equal quadrants and allows users to
// navigate by repeatedly selecting quadrants until reaching a minimum size threshold.
//
// Key Features:
//   - Recursive division: Each selection narrows the active area to 1/4 of the previous
//   - Configurable limits: Minimum size and maximum depth constraints
//   - Backtracking: Support for undoing selections via backspace
//   - warpd-compatible: Uses u/i/j/k key mapping by default (top-left, top-right, bottom-left, bottom-right)
//
// Basic Usage:
//
//	// Create a new quad-grid for a 1920x1080 screen
//	bounds := image.Rect(0, 0, 1920, 1080)
//	grid := quadgrid.NewQuadGrid(bounds, 25, 10) // 25px min, 10 max depth
//
//	// Select top-left quadrant (key 'u')
//	center, complete := grid.SelectQuadrant(quadgrid.TopLeft)
//
//	// center is the cursor position to move to
//	// complete is true if minimum size has been reached
//
// Manager Usage:
//
//	// Create a manager with callbacks
//	manager := quadgrid.NewManager(
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
//   - Customizable via 4-character string
//
// Exit Conditions:
//   - Quadrant size < minimum size (default 25px)
//   - Maximum recursion depth reached (default 10)
//   - User presses exit key
//
// The package is designed to integrate with the Neru mode system and follows
// the same patterns as the existing grid and hints modes.
package quadgrid
