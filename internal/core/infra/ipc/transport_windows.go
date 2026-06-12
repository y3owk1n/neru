//go:build windows

// internal/core/infra/ipc/transport_windows.go
// Named-pipe transport for IPC on Windows via go-winio.
// Does not implement Unix-domain socket cleanup or permissions.

package ipc

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

const pipePath = `\\.\pipe\neru`

func endpointPath() string {
	return pipePath
}

func prepareEndpoint(_ string) error {
	return nil
}

func listenEndpoint(ctx context.Context, path string) (net.Listener, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return winio.ListenPipe(path, nil)
}

func dialEndpoint(ctx context.Context, dialer net.Dialer, path string) (net.Conn, error) {
	if dialer.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, dialer.Timeout)
		defer cancel()
	}

	return winio.DialPipeContext(ctx, path)
}

func cleanupEndpoint(_ string) error {
	return nil
}
