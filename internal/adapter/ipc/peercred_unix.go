//go:build !windows

package ipc

import (
	"net"
	"os"

	"github.com/y3owk1n/neru/internal/derrors"
)

// authorizePeer refuses a connection whose peer is not this user.
//
// The endpoint's own permissions already keep other accounts out. This is a
// second lock on the same door, asked of the kernel rather than of the
// filesystem, so it still holds if a future change to the endpoint's path or
// mode were to loosen the first one. It costs one getsockopt per connection,
// which is once per CLI command and never on the key path.
func authorizePeer(connection net.Conn) error {
	unixConn, ok := connection.(*net.UnixConn)
	if !ok {
		return derrors.New(
			derrors.CodeIPCFailed,
			"IPC connection is not a unix socket, so its peer cannot be identified",
		)
	}

	uid, uidErr := peerUID(unixConn)
	if uidErr != nil {
		return derrors.Wrap(
			uidErr,
			derrors.CodeIPCFailed,
			"cannot read the credentials of the connecting process",
		)
	}

	if int(uid) != os.Getuid() {
		return derrors.Newf(
			derrors.CodeIPCFailed,
			"connecting process belongs to uid %d rather than to this user",
			uid,
		)
	}

	return nil
}
