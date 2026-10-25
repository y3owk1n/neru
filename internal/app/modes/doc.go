// Package modes implements Neru's navigation modes and the Handler that
// activates them, routes keys to the active one, and exits it.
//
// The modes are hints, grid, recursive grid, bisect, scroll, monitor select,
// and declared modes. When none is active the handler is idle.
package modes
