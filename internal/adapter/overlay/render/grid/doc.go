// Package grid renders the grid overlay: the cells laid over the screen, their
// key labels, and the subgrid drawn inside a chosen cell.
//
// It owns style and drawing only. The grid's geometry and the input matching
// live in internal/domain/grid, and the state one grid session keeps lives in
// internal/app/components/grid.
package grid
