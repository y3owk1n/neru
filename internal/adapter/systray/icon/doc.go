// Package icon holds the tray icon bytes.
//
// It is its own package because go:embed can only reach files under the
// embedding package's own directory, and both the Linux and Windows trays draw
// the same brand icon. Embedding it twice would put the same PNG in the tree
// twice and let the two copies drift.
//
// The paused variants live here for the same reason: macOS ships a second
// hand-drawn template glyph, and the tile the other hosts show while Neru is
// paused is derived from the running tile rather than exported a second time
// (paused.go).
package icon
