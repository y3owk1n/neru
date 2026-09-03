//go:build linux

package ipc

import (
	"net"

	"golang.org/x/sys/unix"
)

// peerUID reports the uid of the process at the other end of connection, as
// recorded by the kernel when the connection was made rather than as claimed by
// anything the peer sends. Linux answers this through SO_PEERCRED.
func peerUID(connection *net.UnixConn) (uint32, error) {
	rawConn, rawErr := connection.SyscallConn()
	if rawErr != nil {
		return 0, rawErr
	}

	var (
		credentials *unix.Ucred
		sockoptErr  error
	)

	controlErr := rawConn.Control(func(fd uintptr) {
		credentials, sockoptErr = unix.GetsockoptUcred(
			int(fd),
			unix.SOL_SOCKET,
			unix.SO_PEERCRED,
		)
	})
	if controlErr != nil {
		return 0, controlErr
	}

	if sockoptErr != nil {
		return 0, sockoptErr
	}

	return credentials.Uid, nil
}
