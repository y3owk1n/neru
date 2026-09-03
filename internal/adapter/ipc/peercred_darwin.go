//go:build darwin

package ipc

import (
	"net"

	"golang.org/x/sys/unix"

	"github.com/y3owk1n/neru/internal/derrors"
)

// xucredVersion is XUCRED_VERSION from <sys/ucred.h>, the layout the fields
// below are read at. Darwin fills it in on every successful call; a different
// value would mean the struct is not the one this code knows, and a uid read
// out of it would be a guess.
const xucredVersion = 0

// peerUID reports the uid of the process at the other end of connection, as
// recorded by the kernel when the connection was made rather than as claimed by
// anything the peer sends. Darwin answers this through LOCAL_PEERCRED.
func peerUID(connection *net.UnixConn) (uint32, error) {
	rawConn, rawErr := connection.SyscallConn()
	if rawErr != nil {
		return 0, rawErr
	}

	var (
		credentials *unix.Xucred
		sockoptErr  error
	)

	controlErr := rawConn.Control(func(fd uintptr) {
		credentials, sockoptErr = unix.GetsockoptXucred(
			int(fd),
			unix.SOL_LOCAL,
			unix.LOCAL_PEERCRED,
		)
	})
	if controlErr != nil {
		return 0, controlErr
	}

	if sockoptErr != nil {
		return 0, sockoptErr
	}

	if credentials.Version != xucredVersion {
		return 0, derrors.Newf(
			derrors.CodeIPCFailed,
			"peer credentials came back in layout version %d rather than %d",
			credentials.Version,
			xucredVersion,
		)
	}

	return credentials.Uid, nil
}
