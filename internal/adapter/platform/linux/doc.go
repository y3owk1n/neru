//go:build linux

// Package linux implements ports.SystemPort for the X11, wlroots-Wayland and
// KDE-Wayland backends, plus the cgo bridge for the native C sources here.
// Which backend is live is decided at runtime by platform.DetectLinuxBackend:
// build tags cannot tell KDE from wlroots, both are linux+Wayland at compile
// time. Files split on x11/wayland and cgo/nocgo; see docs/CROSS_PLATFORM.md.
//
// This doc.go is tagged plain `linux` (not linux && cgo) because the package
// contains .c files, which Go rejects without cgo — an untagged file would
// break analysis on other hosts, and the plain tag still gives CGO_ENABLED=0
// builds a package comment.
package linux
