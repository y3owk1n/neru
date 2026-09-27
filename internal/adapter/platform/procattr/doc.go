// Package procattr keeps Windows from giving a console window to a child
// process Neru spawns.
//
// Neru starts a subprocess for its output or its exit status, never to put
// something on screen. On Windows a console program gets a fresh console window
// whenever its parent has no console of its own to inherit, which is the
// daemon's normal state. Every such subprocess then flashes a window the user
// never asked for. The window lives only as long as the command, which is still
// long enough for a tiling window manager to tile it. One creation flag
// suppresses it on Windows. No other platform needs anything.
package procattr
