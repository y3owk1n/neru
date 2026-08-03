// Package icon holds the tray icon bytes.
//
// It is its own package because go:embed can only reach files under the
// embedding package's own directory, and both the Linux and Windows trays draw
// the same brand icon. Embedding it twice would put the same PNG in the tree
// twice and let the two copies drift.
package icon
