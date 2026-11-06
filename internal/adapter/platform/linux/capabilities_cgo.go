//go:build linux && cgo

package linux

// nativeBackendsCompiledIn reports whether the X11 and wlroots client stacks
// were built into this binary. The screen, cursor and process capabilities
// bottom out in them, and a CGO_ENABLED=0 build links _nocgo stubs that answer
// CodeNotSupported on every compositor. Capabilities uses this to report the
// truth rather than the build target's intent.
const nativeBackendsCompiledIn = true
