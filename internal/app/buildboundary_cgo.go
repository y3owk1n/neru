//go:build cgo

package app

// cgoEnabled reports whether this binary was built with cgo. It is the one
// fact announceBuildBoundary cannot learn at runtime: every native backend is
// selected by the same tag, so by the time a call fails the boundary has
// already cost the user a keystroke.
const cgoEnabled = true
