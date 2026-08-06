// Package recursivegrid holds the state one recursive-grid session keeps: the
// selection it has made and the per-activation flags a binding asked for.
//
// It is mode state, not drawing. It lived in the overlay's recursive-grid
// renderer until #1213, which is why the app layer had to name an adapter
// package to say what a mode was doing; nothing here knows a color or a
// surface.
package recursivegrid
