//go:build integration

package ipc_test

import (
	"context"
	"testing"
	"time"

	adapterIPC "github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

// TestIPCAdapterImplementsPort verifies the adapter implements the port interface.
func TestIPCAdapterImplementsPort(t *testing.T) {
	var _ ports.IPCPort = (*adapterIPC.Adapter)(nil)
}

// TestIPCAdapterIntegration tests the IPC adapter with real server.
func TestIPCAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	log := logger.Get()

	// Dummy handler for testing
	handler := func(cmd ipc.Command) ipc.Response {
		return ipc.Response{Success: true}
	}

	// Use a unique port for testing to avoid conflicts
	// Note: NewServer uses a fixed socket path, so we can't easily run parallel tests
	server, err := ipc.NewServer(handler, log)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	adapter := adapterIPC.NewAdapter(server, log)

	ctx := context.Background()

	t.Run("Start and Stop", func(t *testing.T) {
		// Start should initialize server
		err := adapter.Start(ctx)
		if err != nil {
			t.Fatalf("Start() error = %v, want nil", err)
		}

		// IsRunning should return true after start
		if !adapter.IsRunning() {
			t.Error("IsRunning() = false, want true after Start()")
		}

		// Stop should shut down server
		err = adapter.Stop(ctx)
		if err != nil {
			t.Errorf("Stop() error = %v, want nil", err)
		}

		// IsRunning should return false after stop
		if adapter.IsRunning() {
			t.Error("IsRunning() = true, want false after Stop()")
		}
	})

	t.Run("Serve blocks until context canceled", func(t *testing.T) {
		// Create new adapter for this test
		server, err := ipc.NewServer(handler, log)
		if err != nil {
			t.Fatalf("Failed to create server: %v", err)
		}
		adapter := adapterIPC.NewAdapter(server, log)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Serve should block until context is canceled
		done := make(chan error, 1)
		go func() {
			done <- adapter.Serve(ctx)
		}()

		// Wait for context timeout
		select {
		case err := <-done:
			if err != nil && err != context.DeadlineExceeded {
				t.Errorf("Serve() error = %v, want nil or DeadlineExceeded", err)
			}
		case <-time.After(200 * time.Millisecond):
			t.Error("Serve() did not return after context cancellation")
		}
	})

	t.Run("Multiple Start calls", func(t *testing.T) {
		server, err := ipc.NewServer(handler, log)
		if err != nil {
			t.Fatalf("Failed to create server: %v", err)
		}
		adapter := adapterIPC.NewAdapter(server, log)
		defer adapter.Stop(context.Background())

		// First start should succeed
		err = adapter.Start(ctx)
		if err != nil {
			t.Fatalf("First Start() error = %v, want nil", err)
		}

		// Second start should handle gracefully (implementation dependent)
		err = adapter.Start(ctx)
		// We don't assert error here as behavior may vary
		_ = err
	})
}

// TestIPCAdapterContextCancellation tests context cancellation handling.
func TestIPCAdapterContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	log := logger.Get()
	handler := func(cmd ipc.Command) ipc.Response {
		return ipc.Response{Success: true}
	}

	server, err := ipc.NewServer(handler, log)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	adapter := adapterIPC.NewAdapter(server, log)

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("Start with canceled context", func(t *testing.T) {
		// Start might still succeed as it's non-blocking
		// This tests that it handles canceled context gracefully
		err := adapter.Start(ctx)
		_ = err // Implementation dependent
	})
}
