// Package hotkey binds keys to action sequences.
//
// It owns global hotkey registration, per-mode overrides, and the held-key
// repeat loop — including the repeat bookkeeping that used to sit on the App as
// four fields nothing else touched.
//
// The interfaces it depends on are declared here rather than imported from the
// components that satisfy them, and each names two or three methods. That is
// what lets the binder be built and tested without a daemon: it drives modes,
// application state and the focused-app lookup without importing any of them.
package hotkey
