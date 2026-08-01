//go:build linux && !cgo

package linux

// nativeBackendsCompiledIn reports whether the X11 and wlroots client stacks
// were built into this binary. See the cgo variant for why this gates the
// screen, cursor and process capabilities.
const nativeBackendsCompiledIn = false
