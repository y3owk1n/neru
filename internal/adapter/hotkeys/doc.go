// Package hotkeys registers global keyboard shortcuts.
//
// Each platform's Manager implements ports.HotkeyPort directly, so there is no
// wrapper adapter here — only the shared HotkeyID and Callback aliases and a
// build-tagged factory that picks the implementation and registers it as the
// process-wide manager.
//
// The mechanisms differ by platform, and each lives in its own subpackage:
// per-hotkey CGEventTaps on macOS, X11 key grabs or the evdev keyboard proxy on
// Linux depending on the session, and RegisterHotKey on a dedicated message
// thread on Windows — the last through the registry in
// internal/adapter/platform/windows.
package hotkeys
