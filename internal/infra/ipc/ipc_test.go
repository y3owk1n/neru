//nolint:errcheck,errchkjson,revive
package ipc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestGetSocketPath(t *testing.T) {
	path := GetSocketPath()

	if path == "" {
		t.Error("GetSocketPath() returned empty string")
	}

	// Verify path contains expected components
	if len(path) < 10 {
		t.Errorf("GetSocketPath() returned suspiciously short path: %s", path)
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient()

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.socketPath == "" {
		t.Error("Client socket path is empty")
	}
}

func TestIsServerRunning(t *testing.T) {
	// Test when server is not running
	running := IsServerRunning()

	// We can't assert the value since it depends on system state
	// Just verify it doesn't panic
	_ = running
}

func TestNewServer(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name    string
		handler CommandHandler
		wantErr bool
	}{
		{
			name: "valid handler",
			handler: func(_ context.Context, cmd Command) Response {
				return Response{Success: true}
			},
			wantErr: false,
		},
		{
			name:    "nil handler",
			handler: nil,
			wantErr: false, // nil handler is allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewServer(tt.handler, logger)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && server == nil {
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

	handler := func(_ context.Context, _ Command) Response {
		return Response{
			Success: true,
			Message: "test response",
		}
	}

	server, err := NewServer(handler, logger)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	// Start server
	server.Start()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running
	if !IsServerRunning() {
		t.Error("Server should be running after Start()")
	}

	// Stop server
	err = server.Stop()
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Give server time to stop
	time.Sleep(100 * time.Millisecond)

	// Verify server is not running
	if IsServerRunning() {
		t.Error("Server should not be running after Stop()")
	}
}

func TestClientSend(t *testing.T) {
	logger := zap.NewNop()

	// Create test handler
	handler := func(_ context.Context, cmd Command) Response {
		return Response{
			Success: true,
			Message: "test response",
			Data: map[string]string{
				"action": cmd.Action,
			},
		}
	}

	// Start server
	server, err := NewServer(handler, logger)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}
	defer server.Stop()

	server.Start()
	time.Sleep(100 * time.Millisecond)

	// Create client
	client := NewClient()

	tests := []struct {
		name    string
		cmd     Command
		wantErr bool
	}{
		{
			name: "simple command",
			cmd: Command{
				Action: "test",
			},
			wantErr: false,
		},
		{
			name: "command with params",
			cmd: Command{
				Action: "test",
				Params: map[string]any{
					"key": "value",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.Send(tt.cmd)

			if (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if !resp.Success {
					t.Errorf("Send() response not successful: %v", resp)
				}

				if resp.Message != "test response" {
					t.Errorf("Send() unexpected message: %s", resp.Message)
				}
			}
		})
	}
}

func TestClientSendWithTimeout(t *testing.T) {
	logger := zap.NewNop()

	// Create slow handler
	handler := func(_ context.Context, _ Command) Response {
		time.Sleep(200 * time.Millisecond)
		return Response{Success: true}
	}

	// Start server
	server, err := NewServer(handler, logger)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}
	defer server.Stop()

	server.Start()
	time.Sleep(100 * time.Millisecond)

	client := NewClient()

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := Command{Action: "test"}
			_, err := client.SendWithTimeout(cmd, tt.timeout)

			if (err != nil) != tt.wantErr {
				t.Errorf("SendWithTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommandJSON(t *testing.T) {
	cmd := Command{
		Action: "test",
		Params: map[string]any{
			"key": "value",
		},
		Args: []string{"arg1", "arg2"},
	}

	// Marshal
	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	// Unmarshal
	var decoded Command
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
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
	resp := Response{
		Success: true,
		Message: "test message",
		Code:    "success",
		Data: map[string]string{
			"key": "value",
		},
	}

	// Marshal
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	// Unmarshal
	var decoded Response
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}

	// Verify
	if decoded.Success != resp.Success {
		t.Errorf("Success mismatch: got %v, want %v", decoded.Success, resp.Success)
	}

	if decoded.Message != resp.Message {
		t.Errorf("Message mismatch: got %s, want %s", decoded.Message, resp.Message)
	}

	if decoded.Code != resp.Code {
		t.Errorf("Code mismatch: got %s, want %s", decoded.Code, resp.Code)
	}
}

// Benchmark tests.
func BenchmarkClientSend(b *testing.B) {
	logger := zap.NewNop()

	handler := func(_ context.Context, _ Command) Response {
		return Response{Success: true}
	}

	server, _ := NewServer(handler, logger)
	defer server.Stop()

	server.Start()
	time.Sleep(100 * time.Millisecond)

	client := NewClient()
	cmd := Command{Action: "test"}

	b.ResetTimer()
	for b.Loop() {
		_, _ = client.Send(cmd)
	}
}

func BenchmarkJSONMarshal(b *testing.B) {
	cmd := Command{
		Action: "test",
		Params: map[string]any{
			"key": "value",
		},
	}

	for b.Loop() {
		_, _ = json.Marshal(cmd)
	}
}
