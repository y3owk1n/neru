// Package linux registers global hotkeys on linux.
//
// The whole directory is linux-only, so the directory carries the platform and
// the filenames do not repeat it. Manager implements ports.HotkeyPort directly;
// there is no wrapper adapter.
package linux
