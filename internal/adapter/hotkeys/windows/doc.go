// Package windows registers global hotkeys on windows.
//
// The whole directory is windows-only, so the directory carries the platform and
// the filenames do not repeat it. Manager implements ports.HotkeyPort directly;
// there is no wrapper adapter.
package windows
