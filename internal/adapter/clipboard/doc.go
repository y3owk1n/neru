// Package clipboard writes text to the system clipboard.
//
// It backs the copy_text action. The shared adapter bounds every write with a
// deadline and drops NUL bytes, which no clipboard path carries. Only the
// final write differs per platform.
//
// Platform slots: clipboard_darwin.go (pbcopy), clipboard_linux.go (wl-copy on
// Wayland, else xclip or xsel), clipboard_windows.go (the Win32 clipboard), and
// clipboard_other.go, which returns CodeNotSupported. clipboard_unix.go runs
// the copier command for macOS and Linux.
package clipboard
