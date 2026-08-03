// Package mousestate tracks which mouse buttons Neru is currently holding down.
//
// Press and release are separate user actions (drag workflows bind them to
// different keys), so every backend needs to remember what it pressed: to know
// whether a toggle should press or release, to emit drag events while a button
// is held, and to release everything when Neru returns to idle. The tracker is
// shared by the platform adapters so that bookkeeping is identical everywhere.
package mousestate
