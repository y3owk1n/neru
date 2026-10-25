// Package ipc carries commands between the neru CLI and the daemon over the
// per-user endpoint that internal/ipcendpoint names. Server is the daemon side,
// Client is the CLI side, and Adapter exposes the Server as ports.IPCPort.
package ipc
