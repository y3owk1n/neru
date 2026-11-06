//go:build linux

// Package linux implements ports.SystemPort for the X11, wlroots-Wayland and
// KDE-Wayland backends, plus the cgo bridge for the native C sources here.
// Which backend is live is decided at runtime by platform.DetectLinuxBackend.
// Build tags cannot tell KDE from wlroots, both are linux+Wayland at compile
// time. Files split on x11/wayland and cgo/nocgo, as
// docs/contributing/porting.md describes.
//
// This file is tagged plain `linux`, not linux && cgo, on purpose. The .c
// files make Go reject the package without cgo, so an untagged file would break
// analysis on other hosts, and the plain tag still gives CGO_ENABLED=0 builds a
// package comment.
package linux
