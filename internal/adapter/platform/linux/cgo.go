//go:build linux && cgo

// Compiles the native C bridge sources in this directory as the single unit
// other packages link against by blank-importing this package. The package
// comment lives in doc.go, tagged plain `linux` so CGO_ENABLED=0 builds still
// have one.

package linux

/*
#cgo linux pkg-config: x11 xtst xrandr wayland-client xkbcommon cairo xrender xfixes xext fontconfig
*/
import "C"
