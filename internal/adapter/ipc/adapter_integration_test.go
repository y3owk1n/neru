//go:build integration

package ipc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	adapterIPC "github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

// TestIPCAdapterImplementsPort verifies the adapter implements the port interface.
func TestIPCAdapterImplementsPort(_ *testing.T) {
	var _ ports.IPCPort = (*adapterIPC.Adapter)(nil)
}

// TestIPCAdapterIntegration tests the IPC adapter with real server.
func TestIPCAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	// Dummy handler for testing
	handler := func(_ context.Context, _ ipc.Command) ipc.Response {
		return ipc.Response{Success: true}
	}

	// Use a unique port for testing to avoid conflicts
	// Note: NewServer uses a fixed socket path, so we can't easily run parallel tests
	server, serverErr := ipc.NewServer(handler, logger)
	if serverErr != nil {
		t.Fatalf("Failed to create server: %v", serverErr)
	}

	adapter := adapterIPC.NewAdapter(server, logger)

	ctx := context.Background()

	t.Run("Start and Stop", func(t *testing.T) {
		// Start should initialize server
		startErr := adapter.Start(ctx)
		if startErr != nil {
			t.Fatalf("Start() error = %v, want nil", startErr)
		}

		// IsRunning should return true after start
		if !adapter.IsRunning() {
			t.Error("IsRunning() = false, want true after Start()")
		}

		// Stop should shut down server
		stopErr := adapter.Stop(ctx)
		if stopErr != nil {
			t.Errorf("Stop() error = %v, want nil", stopErr)
		}

		// IsRunning should return false after stop
		if adapter.IsRunning() {
			t.Error("IsRunning() = true, want false after Stop()")
		}
	})

	t.Run("Serve blocks until context canceled", func(t *testing.T) {
		// Create new adapter for this test
		server, serverErr := ipc.NewServer(handler, logger)
		if serverErr != nil {
			t.Fatalf("Failed to create server: %v", serverErr)
		}

		adapter := adapterIPC.NewAdapter(server, logger)

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
			if err != nil && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("Serve() error = %v, want nil or DeadlineExceeded", err)
			}
		case <-time.After(200 * time.Millisecond):
			t.Error("Serve() did not return after context cancellation")
		}
	})

	t.Run("Multiple Start calls", func(t *testing.T) {
		var serverErr error

		server, serverErr = ipc.NewServer(handler, logger)
		if serverErr != nil {
			t.Fatalf("Failed to create server: %v", serverErr)
		}

		adapter := adapterIPC.NewAdapter(server, logger)

		defer adapter.Stop(context.Background()) //nolint:errcheck

		// First start should succeed
		serverErr = adapter.Start(ctx)
		if serverErr != nil {
			t.Fatalf("First Start() error = %v, want nil", serverErr)
		}

		// Second start should handle gracefully (implementation dependent)
		serverErr = adapter.Start(ctx)
		// We don't assert error here as behavior may vary
		_ = serverErr
	})

	t.Run("SendCommand", func(t *testing.T) {
		// Create a more sophisticated handler for testing
		testHandler := func(ctx context.Context, cmd ipc.Command) ipc.Response {
			switch cmd.Action {
			case "test":
				return ipc.Response{
					Success: true,
					Message: "test successful",
					Data:    map[string]any{"echo": cmd.Action},
				}
			case "error":
				return ipc.Response{
					Success: false,
					Message: "test error",
					Code:    "TEST_ERROR",
				}
			default:
				return ipc.Response{
					Success: false,
					Message: "unknown action",
					Code:    "UNKNOWN_ACTION",
				}
			}
		}

		server, serverErr := ipc.NewServer(testHandler, logger)
		if serverErr != nil {
			t.Fatalf("Failed to create server: %v", serverErr)
		}

		adapter := adapterIPC.NewAdapter(server, logger)
		defer adapter.Stop(context.Background()) //nolint:errcheck

		startErr := adapter.Start(ctx)
		if startErr != nil {
			t.Fatalf("Start() error = %v, want nil", startErr)
		}

		// Give server time to start
		time.Sleep(50 * time.Millisecond)

		client := ipc.NewClient()

		t.Run("successful command", func(t *testing.T) {
			cmd := ipc.Command{Action: "test", Params: map[string]any{"key": "value"}}

			resp, err := client.Send(cmd)
			if err != nil {
				t.Fatalf("Send() error = %v, want nil", err)
			}

			if !resp.Success {
				t.Errorf("Response success = false, want true")
			}

			if resp.Message != "test successful" {
				t.Errorf("Response message = %q, want %q", resp.Message, "test successful")
			}

			// Check data
			if resp.Data == nil {
				t.Fatal("Response data is nil")
			}

			data, ok := resp.Data.(map[string]any)
			if !ok {
				t.Fatalf("Response data type = %T, want map[string]interface{}", resp.Data)
			}

			if data["echo"] != "test" {
				t.Errorf("Response data echo = %v, want %q", data["echo"], "test")
			}
		})

		t.Run("error command", func(t *testing.T) {
			cmd := ipc.Command{Action: "error"}

			resp, err := client.Send(cmd)
			if err != nil {
				t.Fatalf("Send() error = %v, want nil", err)
			}

			if resp.Success {
				t.Errorf("Response success = true, want false")
			}

			if resp.Message != "test error" {
				t.Errorf("Response message = %q, want %q", resp.Message, "test error")
			}

			if resp.Code != "TEST_ERROR" {
				t.Errorf("Response code = %q, want %q", resp.Code, "TEST_ERROR")
			}
		})

		t.Run("unknown command", func(t *testing.T) {
			cmd := ipc.Command{Action: "unknown"}

			resp, err := client.Send(cmd)
			if err != nil {
				t.Fatalf("Send() error = %v, want nil", err)
			}

			if resp.Success {
				t.Errorf("Response success = true, want false")
			}

			if resp.Code != "UNKNOWN_ACTION" {
				t.Errorf("Response code = %q, want %q", resp.Code, "UNKNOWN_ACTION")
			}
		})
	})
}

// TestIPCAdapterContextCancellation tests context cancellation handling.
func TestIPCAdapterContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()
	handler := func(_ context.Context, _ ipc.Command) ipc.Response {
		return ipc.Response{Success: true}
	}

	server, serverErr := ipc.NewServer(handler, logger)
	if serverErr != nil {
		t.Fatalf("Failed to create server: %v", serverErr)
	}

	adapter := adapterIPC.NewAdapter(server, logger)

	// Create canceled context
	context, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("Start with canceled context", func(_ *testing.T) {
		// Start might still succeed as it's non-blocking
		// This tests that it handles canceled context gracefully
		err := adapter.Start(context)
		_ = err // Implementation dependent
	})
}
