// Package windows provides Windows-specific platform implementations.
//
// It implements ports.SystemPort over pure-Go Win32 bindings (no CGO):
// monitor enumeration, cursor warp and SendInput injection, the
// WH_KEYBOARD_LL hook, RegisterHotKey, layered-window overlays with GDI, and
// dark-mode detection through the personalization registry. UI Automation
// accessibility lives in internal/adapter/accessibility/native/windows.
//
// Windows is the newest backend, so a few capabilities are still stubs — see
// the windows entries in ports.WindowsCapabilities for the authoritative list
// and docs/CROSS_PLATFORM.md for the gap tracker. Stubs must return
// derrors.CodeNotSupported rather than silently no-op.
//
// All functional code in this package carries //go:build windows. This file is
// intentionally untagged so that `go vet ./...` and other analysis tools can
// resolve the package on every OS without hitting "build constraints exclude
// all Go files" — matching the sibling darwin package.
package windows
