// Package mouse implements ports.MousePort: clicks, presses and releases at a
// point, and scrolls at the cursor.
//
// The rules every platform shares live in adapter.go: a left click ends a held
// left drag, other clicks and scrolls release held buttons first, and a toggle
// follows the buttons this process holds. Each platform supplies only its
// injection functions, in backend_darwin.go, backend_linux.go,
// backend_windows.go and backend_other.go.
package mouse
