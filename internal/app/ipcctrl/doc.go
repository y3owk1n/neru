// Package ipcctrl routes IPC commands to application behavior. It is the daemon
// side of the protocol whose transport lives in internal/adapter/ipc.
// Controller takes its dependencies as a Deps struct, so the commands
// run without a daemon. A nil dependency makes the commands needing it
// report unavailable. Sub-handlers carry a Handler suffix.
package ipcctrl
