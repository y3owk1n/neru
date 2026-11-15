// Package scrollanim spreads a scroll over time in eased steps, for the Linux
// and Windows input adapters.
//
// Both adapters animate a smooth scroll the same way, and only the session a
// scroll opens differs: libei, a wlroots virtual pointer, uinput or XTest on
// Linux, SendInput on Windows. Each passes its own way to open one. macOS keeps
// an animator of its own, because it posts each step as a scroll event at a
// point rather than through a session.
package scrollanim
