// Package recursivegrid divides the screen into a grid and narrows the active
// area with every cell the user selects, until the cell is too small to divide
// or the depth limit is hit. Backspace backtracks one level.
//
// The key mapping is a string of grid_cols*grid_rows characters, "rtyfghvbn"
// by default for 3x3. NewRecursiveGrid drives one grid; Manager wires it to
// the mode system with overlay and completion callbacks.
package recursivegrid
