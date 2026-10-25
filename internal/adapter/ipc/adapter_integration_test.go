//go:build integration

package ipc_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
)

// TestIPCAdapterIntegration tests the IPC adapter with real server.
func TestIPCAdapterIntegration(t *testing.T) {
	log := zap.NewNop()

	handler := func(_ context.Context, _ ipc.Command) ipc.Response {
		return ipc.Response{Success: true}
	}

	// NewServer uses a fixed socket path, so these tests cannot run in parallel.
	server, serverErr := ipc.NewServer(handler, log)
	if serverErr != nil {
		t.Fatalf("Failed to create server: %v", serverErr)
	}

	adapter := ipc.NewAdapter(server, log)

	ctx := context.Background()

	t.Run("Start and Stop", func(t *testing.T) {
		startErr := adapter.Start(ctx)
		if startErr != nil {
			t.Fatalf("Start() error = %v, want nil", startErr)
		}

		if !adapter.IsRunning() {
			t.Error("IsRunning() = false, want true after Start()")
		}

		stopErr := adapter.Stop(ctx)
		if stopErr != nil {
			t.Errorf("Stop() error = %v, want nil", stopErr)
		}

		if adapter.IsRunning() {
			t.Error("IsRunning() = true, want false after Stop()")
		}
	})

	t.Run("Multiple Start calls", func(t *testing.T) {
		var serverErr error

		server, serverErr = ipc.NewServer(handler, log)
		if serverErr != nil {
			t.Fatalf("Failed to create server: %v", serverErr)
		}

		adapter := ipc.NewAdapter(server, log)

		defer adapter.Stop(context.Background()) //nolint:errcheck

		serverErr = adapter.Start(ctx)
		if serverErr != nil {
			t.Fatalf("First Start() error = %v, want nil", serverErr)
		}

		// Start is documented as idempotent. It returns nil when the server is
		// already running rather than erroring or starting a second listener.
		// The daemon relies on that during config reload, which re-runs the
		// startup phase without tearing the socket down.
		serverErr = adapter.Start(ctx)
		if serverErr != nil {
			t.Errorf("second Start() error = %v, want nil (Start must be idempotent)", serverErr)
		}

		// A single Stop must still fully stop it. The second Start must not
		// have incremented a reference count that leaves the socket open.
		serverErr = adapter.Stop(ctx)
		if serverErr != nil {
			t.Fatalf("Stop() error = %v, want nil", serverErr)
		}

		// Stop is idempotent too.
		serverErr = adapter.Stop(ctx)
		if serverErr != nil {
			t.Errorf("second Stop() error = %v, want nil (Stop must be idempotent)", serverErr)
		}

		// And the adapter must be restartable after a clean stop.
		serverErr = adapter.Start(ctx)
		if serverErr != nil {
			t.Errorf("Start() after Stop() error = %v, want nil", serverErr)
		}
	})
}

// TestIPCAdapterContextCancellation tests context cancellation handling.
func TestIPCAdapterContextCancellation(t *testing.T) {
	log := zap.NewNop()
	handler := func(_ context.Context, _ ipc.Command) ipc.Response {
		return ipc.Response{Success: true}
	}

	server, serverErr := ipc.NewServer(handler, log)
	if serverErr != nil {
		t.Fatalf("Failed to create server: %v", serverErr)
	}

	adapter := ipc.NewAdapter(server, log)
	defer adapter.Stop(context.Background()) //nolint:errcheck

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("Start with canceled context", func(t *testing.T) {
		// Start should handle a canceled context gracefully (may succeed or return context.Canceled)
		err := adapter.Start(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("Start with canceled context returned unexpected error: %v", err)
		}
	})
}
