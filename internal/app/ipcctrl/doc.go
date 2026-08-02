// Package ipcctrl routes IPC commands to application behavior.
//
// It is the daemon side of the CLI/daemon protocol: internal/adapter/ipc owns
// the transport and the wire types, and this package owns what each command
// does. The split matters because the controller takes its dependencies as a
// Deps struct rather than reaching for the App — that is what lets the command
// surface be tested without building a daemon, and what let this package be
// lifted out of internal/app at all.
//
// Sub-handlers carry a Handler suffix (ModesHandler, ScrollHandler, ...) so a
// handler never collides with the Controller field naming the same concept.
package ipcctrl
