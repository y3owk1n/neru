//go:build integration

package cli_test

import (
	"context"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
)

// waitForServerReady polls the IPC server until it's ready or times out
func waitForServerReady(t *testing.T, timeout time.Duration) {
	t.Helper()
	client := ipc.NewClient()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		_, err := client.Send(ipc.Command{Action: "ping"})
		if err == nil {
			return // Server is ready
		}
		time.Sleep(10 * time.Millisecond) // Short poll interval
	}

	t.Fatalf("Server did not become ready within %v", timeout)
}

// mockAppState simulates app state for testing
type mockAppState struct {
	running bool
	mode    string
	started bool
}

func newMockAppState() *mockAppState {
	return &mockAppState{
		running: true, // Start in running state to match test expectations
		mode:    "idle",
		started: false,
	}
}

// TestCLIIntegration tests IPC communication with real infrastructure
func TestCLIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()
	appState := newMockAppState()

	// Create a real IPC server with handlers that simulate app behavior
	handler := func(ctx context.Context, cmd ipc.Command) ipc.Response {
		switch cmd.Action {
		case "ping":
			return ipc.Response{Success: true, Data: map[string]interface{}{"status": "ok"}}
		case "start":
			appState.started = true
			appState.running = true
			return ipc.Response{Success: true, Data: map[string]interface{}{"message": "started"}}
		case "stop":
			appState.running = false
			return ipc.Response{Success: true, Data: map[string]interface{}{"message": "stopped"}}
		case "status":
			return ipc.Response{Success: true, Data: map[string]interface{}{
				"running": appState.running,
				"mode":    appState.mode,
			}}
		case "hints":
			if !appState.running {
				return ipc.Response{Success: false, Message: "app not running"}
			}
			appState.mode = "hints"
			return ipc.Response{Success: true, Data: map[string]interface{}{"mode": "hints"}}
		case "grid":
			if !appState.running {
				return ipc.Response{Success: false, Message: "app not running"}
			}
			appState.mode = "grid"
			return ipc.Response{Success: true, Data: map[string]interface{}{"mode": "grid"}}
		case "action":
			if !appState.running {
				return ipc.Response{Success: false, Message: "app not running"}
			}
			if len(cmd.Args) >= 3 && cmd.Args[0] == "left_click" {
				return ipc.Response{Success: true, Message: "action performed"}
			}
			return ipc.Response{Success: false, Message: "invalid action"}
		case "idle":
			if !appState.running {
				return ipc.Response{Success: false, Message: "app not running"}
			}
			appState.mode = "idle"
			return ipc.Response{Success: true, Data: map[string]interface{}{"mode": "idle"}}
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

	// Wait for server to be ready
	waitForServerReady(t, 2*time.Second)

	t.Run("CLI ping command", func(t *testing.T) {
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
		if mode, ok := data["mode"]; !ok || mode != "idle" {
			t.Errorf("Expected mode='idle', got %v", mode)
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

	t.Run("CLI action command", func(t *testing.T) {
		client := ipc.NewClient()

		// Test left click action
		response, err := client.Send(ipc.Command{
			Action: "action",
			Args:   []string{"left_click", "100", "100"},
		})
		if err != nil {
			t.Fatalf("Failed to send action: %v", err)
		}

		if !response.Success {
			t.Errorf("Action should succeed: %v", response.Message)
		}
		if response.Message != "action performed" {
			t.Errorf("Expected message 'action performed', got %q", response.Message)
		}
	})

	t.Run("CLI stop command", func(t *testing.T) {
		client := ipc.NewClient()

		response, err := client.Send(ipc.Command{Action: "stop"})
		if err != nil {
			t.Fatalf("Failed to send stop: %v", err)
		}

		if !response.Success {
			t.Errorf("Stop failed: %v", response.Message)
		}
	})
}
