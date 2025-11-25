package ipc_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/infra/ipc"
	"go.uber.org/zap"
)

func TestGetSocketPath(t *testing.T) {
	path := ipc.GetSocketPath()

	if path == "" {
		t.Error("GetSocketPath() returned empty string")
	}

	// Verify path contains expected components
	if len(path) < 10 {
		t.Errorf("GetSocketPath() returned suspiciously short path: %s", path)
	}
}

func TestNewClient(t *testing.T) {
	client := ipc.NewClient()

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.GetSocketPath() == "" {
		t.Error("Client socket path is empty")
	}
}

func TestIsServerRunning(_ *testing.T) {
	// Test when server is not running
	running := ipc.IsServerRunning()

	// We can't assert the value since it depends on system state
	// Just verify it doesn't panic
	_ = running
}

func TestNewServer(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name    string
		handler ipc.CommandHandler
		wantErr bool
	}{
		{
			name: "valid handler",
			handler: func(_ context.Context, _ ipc.Command) ipc.Response {
				return ipc.Response{Success: true}
			},
			wantErr: false,
		},
		{
			name:    "nil handler",
			handler: nil,
			wantErr: false, // nil handler is allowed
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, err := ipc.NewServer(test.handler, logger)

			if (err != nil) != test.wantErr {
				t.Errorf("NewServer() error = %v, wantErr %v", err, test.wantErr)

				return
			}

			if !test.wantErr && server == nil {
				t.Error("NewServer() returned nil server")
			}

			// Clean up
			if server != nil {
				_ = server.Stop()
			}
		})
	}
}

func TestServerStartStop(t *testing.T) {
	logger := zap.NewNop()

	handler := func(_ context.Context, _ ipc.Command) ipc.Response {
		return ipc.Response{
			Success: true,
			Message: "test response",
		}
	}

	server, serverErr := ipc.NewServer(handler, logger)
	if serverErr != nil {
		t.Fatalf("NewServer() failed: %v", serverErr)
	}

	// Start server
	server.Start()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running
	if !ipc.IsServerRunning() {
		t.Error("Server should be running after Start()")
	}

	// Stop server
	serverErr = server.Stop()
	if serverErr != nil {
		t.Errorf("Stop() failed: %v", serverErr)
	}

	// Give server time to stop
	time.Sleep(100 * time.Millisecond)

	// Verify server is not running
	if ipc.IsServerRunning() {
		t.Error("Server should not be running after Stop()")
	}
}

func TestClientSend(t *testing.T) {
	logger := zap.NewNop()

	// Create test handler
	handler := func(_ context.Context, cmd ipc.Command) ipc.Response {
		return ipc.Response{
			Success: true,
			Message: "test response",
			Data: map[string]string{
				"action": cmd.Action,
			},
		}
	}

	// Start server
	server, serverErr := ipc.NewServer(handler, logger)
	if serverErr != nil {
		t.Fatalf("NewServer() failed: %v", serverErr)
	}

	defer func() {
		_ = server.Stop()
	}()

	server.Start()
	time.Sleep(100 * time.Millisecond)

	// Create client
	client := ipc.NewClient()

	tests := []struct {
		name    string
		cmd     ipc.Command
		wantErr bool
	}{
		{
			name: "simple command",
			cmd: ipc.Command{
				Action: "test",
			},
			wantErr: false,
		},
		{
			name: "command with params",
			cmd: ipc.Command{
				Action: "test",
				Params: map[string]any{
					"key": "value",
				},
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ipcResponse, ipcResponseErr := client.Send(test.cmd)

			if (ipcResponseErr != nil) != test.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", ipcResponseErr, test.wantErr)

				return
			}

			if !test.wantErr {
				if !ipcResponse.Success {
					t.Errorf("Send() response not successful: %v", ipcResponse)
				}

				if ipcResponse.Message != "test response" {
					t.Errorf("Send() unexpected message: %s", ipcResponse.Message)
				}
			}
		})
	}
}

func TestClientSendWithTimeout(t *testing.T) {
	logger := zap.NewNop()

	// Create slow handler
	handler := func(_ context.Context, _ ipc.Command) ipc.Response {
		time.Sleep(200 * time.Millisecond)

		return ipc.Response{Success: true}
	}

	// Start server
	server, serverErr := ipc.NewServer(handler, logger)
	if serverErr != nil {
		t.Fatalf("NewServer() failed: %v", serverErr)
	}

	defer func() {
		_ = server.Stop()
	}()

	server.Start()
	time.Sleep(100 * time.Millisecond)

	client := ipc.NewClient()

	tests := []struct {
		name    string
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "timeout too short",
			timeout: 50 * time.Millisecond,
			wantErr: true,
		},
		{
			name:    "timeout sufficient",
			timeout: 500 * time.Millisecond,
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := ipc.Command{Action: "test"}
			_, ipcResponseErr := client.SendWithTimeout(cmd, test.timeout)

			if (ipcResponseErr != nil) != test.wantErr {
				t.Errorf("SendWithTimeout() error = %v, wantErr %v", ipcResponseErr, test.wantErr)
			}
		})
	}
}

func TestCommandJSON(t *testing.T) {
	cmd := ipc.Command{
		Action: "test",
		Params: map[string]any{
			"key": "value",
		},
		Args: []string{"arg1", "arg2"},
	}

	// Marshal
	data, dataErr := json.Marshal(cmd)
	if dataErr != nil {
		t.Fatalf("json.Marshal() failed: %v", dataErr)
	}

	// Unmarshal
	var decoded ipc.Command

	dataErr = json.Unmarshal(data, &decoded)
	if dataErr != nil {
		t.Fatalf("json.Unmarshal() failed: %v", dataErr)
	}

	// Verify
	if decoded.Action != cmd.Action {
		t.Errorf("Action mismatch: got %s, want %s", decoded.Action, cmd.Action)
	}

	if len(decoded.Args) != len(cmd.Args) {
		t.Errorf("Args length mismatch: got %d, want %d", len(decoded.Args), len(cmd.Args))
	}
}

func TestResponseJSON(t *testing.T) {
	response := ipc.Response{
		Success: true,
		Message: "test message",
		Code:    "success",
		Data: map[string]string{
			"key": "value",
		},
	}

	// Marshal
	data, dataErr := json.Marshal(response)
	if dataErr != nil {
		t.Fatalf("json.Marshal() failed: %v", dataErr)
	}

	// Unmarshal
	var decoded ipc.Response

	dataErr = json.Unmarshal(data, &decoded)
	if dataErr != nil {
		t.Fatalf("json.Unmarshal() failed: %v", dataErr)
	}

	// Verify
	if decoded.Success != response.Success {
		t.Errorf("Success mismatch: got %v, want %v", decoded.Success, response.Success)
	}

	if decoded.Message != response.Message {
		t.Errorf("Message mismatch: got %s, want %s", decoded.Message, response.Message)
	}

	if decoded.Code != response.Code {
		t.Errorf("Code mismatch: got %s, want %s", decoded.Code, response.Code)
	}
}
