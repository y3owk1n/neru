// Package stickyindicator renders the badge showing which modifiers are
// currently held sticky.
//
// It owns the overlay's style and drawing only; which modifiers are sticky is
// tracked by the mode handler's modifier state.
//
// Drawing is per-platform (overlay_darwin.go, overlay_linux_*.go,
// overlay_windows.go). This file is untagged so every target has a package
// comment — the comment used to live in overlay_linux_common.go, which meant
// macOS and Windows builds had none.
package stickyindicator
