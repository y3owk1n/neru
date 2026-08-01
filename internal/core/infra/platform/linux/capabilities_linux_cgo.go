//go:build linux && cgo

package linux

// nativeBackendsCompiledIn reports whether the X11 and wlroots client stacks
// were built into this binary.
//
// The screen, cursor and process capabilities all bottom out in those stacks
// (Xlib for X11, the wlr-* Wayland protocols for wlroots), so a CGO_ENABLED=0
// build links the _nocgo stubs instead and every one of them answers
// CodeNotSupported no matter which compositor is running. Capabilities uses
// this to report the truth rather than the build target's intent.
const nativeBackendsCompiledIn = true
