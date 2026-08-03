// Package ipcctrl routes IPC commands to application behavior — the daemon
// side of the protocol whose transport lives in internal/adapter/ipc.
// Controller takes its dependencies as a Deps struct, so the command surface
// runs without a daemon; a nil dependency makes the commands needing it
// report unavailable. Sub-handlers carry a Handler suffix.
package ipcctrl
