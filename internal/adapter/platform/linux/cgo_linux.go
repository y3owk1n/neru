//go:build linux && cgo

// internal/adapter/platform/linux/cgo_linux.go
// Compiles the native C bridge sources (.c) in this directory. Other packages
// include the matching headers and blank-import this package so the linker
// resolves bridge symbols from a single compilation unit.
// The package comment lives in doc.go, which is tagged plain `linux` so that
// CGO_ENABLED=0 builds still have one.

package linux

/*
#cgo linux pkg-config: x11 xtst xrandr wayland-client xkbcommon cairo xrender xfixes xext fontconfig
*/
import "C"
