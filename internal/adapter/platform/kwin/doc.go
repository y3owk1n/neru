// Package kwin holds the one KWin geometry source in the tree.
//
// A Wayland client cannot ask the compositor where another client's window is,
// and KWin exposes no CLI that answers it either. What it does expose is
// scripting: this package installs a small KWin script that pushes the focused
// window's on-screen client geometry to a Neru-owned D-Bus service on every
// focus change, and caches the last value.
//
// Two callers need that same fact and used to be able to answer it separately,
// which is how one of them ended up not answering it at all: the AT-SPI client
// offsets window-relative element coordinates by the window's origin, and
// SystemPort.FocusedWindowBounds reports the focused window's rectangle. Both
// read the process-wide bridge returned by Shared, so KDE has one geometry
// source rather than one per caller.
package kwin
