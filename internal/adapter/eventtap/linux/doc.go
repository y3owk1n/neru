// Package linux captures keys on Linux, and owns the Wayland global-hotkey
// listener and the uinput scroll device that go with it.
//
// Two capture mechanisms live here because Linux needs both: X11 taps the X
// server, Wayland reads evdev directly. Which one runs is decided at runtime
// from the detected backend, not at build time, so both compile into the same
// binary.
package linux
