// Package linux captures keys on Linux, and owns the Wayland keyboard proxy and
// the uinput scroll device that go with it.
//
// Two capture mechanisms live here because Linux needs both: X11 taps the X
// server, Wayland reads evdev directly. Which one runs is decided at runtime
// from the detected backend, not at build time, so both compile into the same
// binary. On Wayland one reader serves both the global hotkeys and the in-mode
// tap: the proxy holds every keyboard and re-emits it through a uinput device,
// so capturing keys for a mode is a routing decision rather than a grab
// (docs/adr/0014-the-wayland-keyboard-is-a-proxy.md).
package linux
