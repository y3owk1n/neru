//go:build linux && cgo

// Package linux compiles native C bridge sources (.c) in this directory.
// Other packages include the matching headers and blank-import this package
// so the linker resolves bridge symbols from a single compilation unit.
package linux

/*
#cgo linux pkg-config: x11 xtst xrandr wayland-client xkbcommon cairo xrender xfixes xext fontconfig
*/
import "C"
