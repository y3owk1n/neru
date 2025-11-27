//go:build integration

package cli_test

import (
	"context"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/infra/ipc"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

// TestCLIIntegration tests CLI commands that communicate with IPC
func TestCLIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	// Create a test IPC server
	handler := func(ctx context.Context, cmd ipc.Command) ipc.Response {
		switch cmd.Action {
		case "ping":
			return ipc.Response{Success: true, Data: map[string]interface{}{"status": "ok"}}
		case "start":
			return ipc.Response{Success: true, Data: map[string]interface{}{"message": "started"}}
		case "stop":
			return ipc.Response{Success: true, Data: map[string]interface{}{"message": "stopped"}}
		case "status":
			return ipc.Response{Success: true, Data: map[string]interface{}{
				"running": true,
				"mode":    "idle",
			}}
		case "hints":
			return ipc.Response{Success: true, Data: map[string]interface{}{"mode": "hints"}}
		case "grid":
			return ipc.Response{Success: true, Data: map[string]interface{}{"mode": "grid"}}
		default:
			return ipc.Response{Success: false, Message: "unknown command"}
		}
	}

	server, err := ipc.NewServer(handler, logger)
	if err != nil {
		t.Fatalf("Failed to create IPC server: %v", err)
	}

	server.Start()
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	t.Run("CLI ping command", func(t *testing.T) {
		// Test that ping command works
		// Note: This would normally be tested by actually running the CLI
		// but for integration testing, we verify the IPC communication works
		client := ipc.NewClient()

		response, err := client.Send(ipc.Command{Action: "ping"})
		if err != nil {
			t.Fatalf("Failed to send ping: %v", err)
		}

		if !response.Success {
			t.Errorf("Ping failed: %v", response.Message)
		}

		data, ok := response.Data.(map[string]interface{})
		if !ok {
			t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
			return
		}
		if status, ok := data["status"]; !ok || status != "ok" {
			t.Errorf("Expected status 'ok', got %v", status)
		}
	})

	t.Run("CLI start command", func(t *testing.T) {
		client := ipc.NewClient()

		response, err := client.Send(ipc.Command{Action: "start"})
		if err != nil {
			t.Fatalf("Failed to send start: %v", err)
		}

		if !response.Success {
			t.Errorf("Start failed: %v", response.Message)
		}
	})

	t.Run("CLI status command", func(t *testing.T) {
		client := ipc.NewClient()

		response, err := client.Send(ipc.Command{Action: "status"})
		if err != nil {
			t.Fatalf("Failed to send status: %v", err)
		}

		if !response.Success {
			t.Errorf("Status failed: %v", response.Message)
		}

		data, ok := response.Data.(map[string]interface{})
		if !ok {
			t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
			return
		}
		if running, ok := data["running"]; !ok || running != true {
			t.Errorf("Expected running=true, got %v", running)
		}
	})

	t.Run("CLI hints command", func(t *testing.T) {
		client := ipc.NewClient()

		response, err := client.Send(ipc.Command{Action: "hints"})
		if err != nil {
			t.Fatalf("Failed to send hints: %v", err)
		}

		if !response.Success {
			t.Errorf("Hints failed: %v", response.Message)
		}

		data, ok := response.Data.(map[string]interface{})
		if !ok {
			t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
			return
		}
		if mode, ok := data["mode"]; !ok || mode != "hints" {
			t.Errorf("Expected mode='hints', got %v", mode)
		}
	})

	t.Run("CLI grid command", func(t *testing.T) {
		client := ipc.NewClient()

		response, err := client.Send(ipc.Command{Action: "grid"})
		if err != nil {
			t.Fatalf("Failed to send grid: %v", err)
		}

		if !response.Success {
			t.Errorf("Grid failed: %v", response.Message)
		}

		data, ok := response.Data.(map[string]interface{})
		if !ok {
			t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
			return
		}
		if mode, ok := data["mode"]; !ok || mode != "grid" {
			t.Errorf("Expected mode='grid', got %v", mode)
		}
	})
}
