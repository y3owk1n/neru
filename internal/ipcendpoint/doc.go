// Package ipcendpoint finds, binds and dials the endpoint the daemon and its
// clients talk over: a Unix socket in a directory only its owner can enter, or
// a Windows named pipe named after its owner and restricted to them.
//
// The daemon, the CLI and the public client package all reach the endpoint
// through here, so the checks on who owns it exist once.
package ipcendpoint
