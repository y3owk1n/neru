//go:build linux

// Package inject posts mouse input on Linux: clicks, presses and releases,
// cursor moves for a click, and scrolls, through XTest on X11, the wlroots
// virtual pointer or libei on Wayland, and the uinput wheel on Hyprland. It
// also holds the modifier presses those need, the X11 key feed that shares
// them, and the slots the event tap and logger are wired into.
//
// It lives beside platform/linux rather than inside it because it imports
// eventtap/linux, which imports platform/linux. internal/adapter/mouse picks
// it as the Linux mouse backend.
package inject
