//go:build !darwin && !linux && !windows

package ipc

import (
	"net"

	"github.com/y3owk1n/neru/internal/derrors"
)

// peerUID has no implementation outside the platforms neru targets. It refuses
// rather than returning a uid it did not read, because authorizePeer would read
// a silent zero as "root connected" and the whole point of the check is that it
// answers from the kernel or not at all.
func peerUID(_ *net.UnixConn) (uint32, error) {
	return 0, derrors.New(
		derrors.CodeNotSupported,
		"reading a socket peer's credentials is not implemented on this platform",
	)
}
