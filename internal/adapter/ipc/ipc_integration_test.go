//go:build integration

package ipc_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
)

const (
	testResponseMessage = "test response"
	testCommandAction   = "test"
	testMapKey          = "key"
	testMapValue        = "value"
	testWireVersion     = "1.2.3"
)

func TestSocketPath(t *testing.T) {
	path := ipc.SocketPath()

	if path == "" {
		t.Error("SocketPath() returned empty string")
	}

	// Verify path contains expected components
	if len(path) < 10 {
		t.Errorf("SocketPath() returned suspiciously short path: %s", path)
	}
}

func TestNewClient(t *testing.T) {
	client := ipc.NewClient()

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.SocketPath() == "" {
		t.Error("Client socket path is empty")
	}
}

// TestIsServerRunning asserts the probe tracks a real server's lifecycle.
//
// It previously called IsServerRunning and discarded the result, so it could
// not fail. The probe reads a process-wide socket, so this skips when a daemon
// already owns it rather than fighting for the path.
func TestIsServerRunning(t *testing.T) {
	if ipc.IsServerRunning() {
		t.Skip("a neru daemon already owns the IPC socket; skipping lifecycle assertions")
	}

	server, err := ipc.NewServer(
		func(_ context.Context, _ ipc.Command) ipc.Response {
			return ipc.Response{Success: true, Code: ipc.CodeOK}
		},
		zap.NewNop(),
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v, want nil", err)
	}

	server.Start()

	t.Cleanup(func() {
		_ = server.Stop()
	})

	// Start is asynchronous, so poll rather than assuming the listener is up.
	deadline := time.Now().Add(3 * time.Second)
	for !ipc.IsServerRunning() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	if !ipc.IsServerRunning() {
		t.Fatal("IsServerRunning() = false while a started server owns the socket")
	}

	err = server.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v, want nil", err)
	}

	deadline = time.Now().Add(3 * time.Second)
	for ipc.IsServerRunning() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	if ipc.IsServerRunning() {
		t.Error("IsServerRunning() = true after the server stopped")
	}
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			server, err := ipc.NewServer(testCase.handler, logger)

			if (err != nil) != testCase.wantErr {
				t.Errorf("NewServer() error = %v, wantErr %v", err, testCase.wantErr)

				return
			}

			if !testCase.wantErr && server == nil {
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
			Message: testResponseMessage,
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
			Message: testResponseMessage,
			Data: map[string]string{
				testMapKey: cmd.Action,
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
				Action: testCommandAction,
			},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			ipcResponse, ipcResponseErr := client.Send(testCase.cmd)

			if (ipcResponseErr != nil) != testCase.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", ipcResponseErr, testCase.wantErr)

				return
			}

			if !testCase.wantErr {
				if !ipcResponse.Success {
					t.Errorf("Send() response not successful: %v", ipcResponse)
				}

				if ipcResponse.Message != testResponseMessage {
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cmd := ipc.Command{Action: testCommandAction}
			_, ipcResponseErr := client.SendWithTimeout(cmd, testCase.timeout)

			if (ipcResponseErr != nil) != testCase.wantErr {
				t.Errorf(
					"SendWithTimeout() error = %v, wantErr %v",
					ipcResponseErr,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestCommandJSON pins the wire keys a Command marshals to. A newer CLI talks
// to an older daemon over this format, so a renamed json tag must fail here
// rather than round-trip cleanly through the same struct.
func TestCommandJSON(t *testing.T) {
	cmd := ipc.Command{
		Version: testWireVersion,
		Action:  testCommandAction,
		Params: map[string]any{
			testMapKey: testMapValue,
		},
		Args: []string{"arg1", "arg2"},
	}

	assertWireKeys(t, cmd, map[string]any{
		"version": testWireVersion,
		"action":  testCommandAction,
		"params":  map[string]any{testMapKey: testMapValue},
		"args":    []any{"arg1", "arg2"},
	})
}

// TestResponseJSON pins the wire keys a Response marshals to, for the same
// cross-version reason as TestCommandJSON.
func TestResponseJSON(t *testing.T) {
	response := ipc.Response{
		Version: testWireVersion,
		Success: true,
		Message: "test message",
		Code:    ipc.CodeOK,
		Data: map[string]string{
			testMapKey: testMapValue,
		},
	}

	assertWireKeys(t, response, map[string]any{
		"version": testWireVersion,
		"success": true,
		"message": "test message",
		"code":    ipc.CodeOK,
		"data":    map[string]any{testMapKey: testMapValue},
	})
}

// assertWireKeys marshals value and checks the decoded JSON object holds
// exactly the want keys with the want values.
func assertWireKeys(t *testing.T, value any, want map[string]any) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	var got map[string]any

	err = json.Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("wire JSON = %s, want keys and values %v", data, want)
	}
}

func TestClientSend_ServerNotRunning(t *testing.T) {
	client := ipc.NewClient()

	cmd := ipc.Command{Action: testCommandAction}

	_, err := client.Send(cmd)
	if err == nil {
		t.Error("Expected error when server is not running")
	}
}

func TestClientSendWithTimeout_ServerNotRunning(t *testing.T) {
	client := ipc.NewClient()

	cmd := ipc.Command{Action: testCommandAction}

	_, err := client.SendWithTimeout(cmd, time.Second)
	if err == nil {
		t.Error("Expected error when server is not running")
	}
}

func TestClient_SocketPath(t *testing.T) {
	client := ipc.NewClient()
	path := client.SocketPath()

	if path == "" {
		t.Error("Client.SocketPath() returned empty string")
	}

	if !filepath.IsAbs(path) {
		t.Errorf("Client.SocketPath() returned relative path: %s", path)
	}
}
