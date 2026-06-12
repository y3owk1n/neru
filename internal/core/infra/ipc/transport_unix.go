//go:build !windows

// internal/core/infra/ipc/transport_unix.go
// Unix-domain socket transport for IPC on darwin and linux.
// Does not implement the Windows named-pipe transport.

package ipc

import (
	"context"
	"net"
	"os"
	"path/filepath"
)

func endpointPath() string {
	return filepath.Join(os.TempDir(), SocketName)
}

func prepareEndpoint(path string) error {
	removeErr := os.Remove(path)
	if removeErr != nil && !os.IsNotExist(removeErr) {
		return removeErr
	}

	return nil
}

func listenEndpoint(ctx context.Context, path string) (net.Listener, error) {
	if err := prepareEndpoint(path); err != nil {
		return nil, err
	}

	listener, err := (&net.ListenConfig{}).Listen(ctx, "unix", path)
	if err != nil {
		return nil, err
	}

	if chmodErr := os.Chmod(path, DefaultSocketPerms); chmodErr != nil {
		_ = listener.Close()

		return nil, chmodErr
	}

	return listener, nil
}

func dialEndpoint(ctx context.Context, dialer net.Dialer, path string) (net.Conn, error) {
	return dialer.DialContext(ctx, "unix", path)
}

func cleanupEndpoint(path string) error {
	removeErr := os.Remove(path)
	if removeErr != nil && !os.IsNotExist(removeErr) {
		return removeErr
	}

	return nil
}
