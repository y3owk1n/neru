//go:build windows

package ipc

import "net"

// authorizePeer is satisfied before the connection exists on Windows.
//
// The named pipe is created with a protected DACL naming this user's SID alone
// (see transport_windows.go), so a connection that reached Accept was already
// opened by a process the kernel checked against that descriptor. There is no
// second question to ask here — go-winio exposes no handle to query the client
// with, and impersonating one would be a wider capability than the check needs.
func authorizePeer(_ net.Conn) error {
	return nil
}
