// Package windows implements ports.SystemPort over pure-Go Win32 bindings, no
// cgo: monitor enumeration, SendInput, the WH_KEYBOARD_LL hook, RegisterHotKey,
// GDI layered-window overlays, dark-mode via the registry. UI Automation lives
// in adapter/accessibility/native/windows.
//
// Anything that talks to a thread-affine Win32 API (the keyboard hook,
// RegisterHotKey, SetWinEventHook, hidden message windows) runs its own
// runtime.LockOSThread message loop; never call those APIs from an arbitrary
// goroutine.
//
// The newest backend, so some capabilities are stubs — ports.WindowsCapabilities
// is the authoritative list. Stubs return derrors.CodeNotSupported, never a
// silent no-op. This file is untagged so analysis resolves the package on
// every OS; everything else carries //go:build windows.
package windows
