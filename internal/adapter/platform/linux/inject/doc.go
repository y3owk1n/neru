//go:build linux

// Package inject posts mouse input on Linux: clicks, presses, releases, cursor
// moves and scrolls. It goes through XTest on X11, the wlroots virtual pointer
// or libei on Wayland, and the uinput wheel on Hyprland. It also holds the
// modifier presses those need, the X11 key feed that shares them, and the
// slots the app wires the event tap and logger into.
//
// It lives beside platform/linux rather than inside it because it imports
// eventtap/linux, which imports platform/linux. internal/adapter/mouse picks
// it as the Linux mouse backend.
package inject
