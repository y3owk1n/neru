//go:build linux

// Package linux provides Linux-specific platform implementations.
//
// It implements ports.SystemPort for the X11, wlroots-Wayland, and KDE-Wayland
// backends, plus the CGO bridge that compiles the native C sources in this
// directory (X11/XTest/XRandR, wayland-client, libei, evdev, uinput, Cairo).
// Which backend is live is decided at runtime by
// platform.DetectLinuxBackend and passed to NewSystemAdapter — build tags
// cannot distinguish compositors, since KDE and wlroots are both linux+Wayland
// at compile time.
//
// Files split on two axes: the compile-time axis (`*_linux_x11.go`,
// `*_linux_wayland.go`, `*_cgo.go` / `*_nocgo.go`) and, inside the Wayland
// files, a runtime switch on the detected compositor. See
// docs/CROSS_PLATFORM.md for the slot rules.
//
// This file carries //go:build linux rather than staying untagged like the
// sibling darwin package's doc.go: the native bridge sources here are .c files,
// which Go rejects outright in a package built without cgo, so an untagged file
// would break analysis on other hosts instead of helping it. The tag is
// deliberately plain `linux` — not `linux && cgo` — so that CGO_ENABLED=0
// builds still get a package comment.
package linux
