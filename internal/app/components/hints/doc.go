// Package hints holds the state one hints session keeps: the collection on
// screen, the search the user is typing, and the per-activation flags a
// binding asked for.
//
// It is mode state, not drawing. It lived in the overlay's hints renderer
// until #1213, which is why the app layer had to name an adapter package to
// say what a mode was doing; nothing here knows a color or a surface.
package hints
