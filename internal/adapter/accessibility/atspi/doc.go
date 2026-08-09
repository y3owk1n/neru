// Package atspi is the Linux accessibility backend. It walks the AT-SPI tree
// over D-Bus for clickable elements and asks the compositor for window
// geometry, since each compositor places AT-SPI coordinates differently — the
// per-compositor files reconcile that. Everything that is not tree walking is
// delegated to the embedded native.Client. Adding a compositor means one file
// here plus a case in window_origin.go: in the wlroots switch when the
// compositor exports an IPC socket of its own, in the backend switch when
// platform.DetectLinuxBackend has a value naming it.
package atspi
