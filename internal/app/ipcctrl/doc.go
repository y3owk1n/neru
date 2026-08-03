// Package ipcctrl routes IPC commands to application behavior.
//
// It is the daemon side of the CLI/daemon protocol: internal/adapter/ipc owns
// the transport and the wire types, and this package owns what each command
// does.
//
// Controller takes its dependencies as a Deps struct rather than reaching for
// the App, so the whole command surface can be exercised without building a
// daemon. A nil dependency is valid and makes the commands that need it report
// that they are unavailable.
//
// Sub-handlers carry a Handler suffix (ModesHandler, ScrollHandler, ...) so a
// handler never collides with the Controller field naming the same concept.
package ipcctrl
