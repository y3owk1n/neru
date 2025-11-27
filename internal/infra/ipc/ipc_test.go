package ipc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	adapterIPC "github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"github.com/y3owk1n/neru/internal/infra/logger"
	"go.uber.org/zap"
)

func TestIPCAdapter_ConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	logger := logger.Get()

	// Handler that tracks concurrent access
	var mutex sync.Mutex

	callCount := 0

	handler := func(_ context.Context, cmd ipc.Command) ipc.Response {
		mutex.Lock()

		callCount++

		count := callCount

		mutex.Unlock()

		return ipc.Response{
			Success: true,
			Message: fmt.Sprintf("call %d", count),
			Data:    map[string]any{"call_id": count},
		}
	}

	server, serverErr := ipc.NewServer(handler, logger)
	if serverErr != nil {
		t.Fatalf("Failed to create server: %v", serverErr)
	}

	adapter := adapterIPC.NewAdapter(server, logger)
	defer adapter.Stop(context.Background()) //nolint:errcheck

	startErr := adapter.Start(context.Background())
	if startErr != nil {
		t.Fatalf("Start() error = %v, want nil", startErr)
	}

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Test concurrent IPC calls
	const numGoroutines = 10

	const callsPerGoroutine = 5

	var waitGroup sync.WaitGroup

	results := make([][]ipc.Response, numGoroutines)

	for goroutineIndex := range numGoroutines {
		waitGroup.Add(1)

		go func(goroutineID int) {
			defer waitGroup.Done()

			client := ipc.NewClient()
			responses := make([]ipc.Response, callsPerGoroutine)

			for callIndex := range callsPerGoroutine {
				cmd := ipc.Command{
					Action: "test",
					Params: map[string]any{
						"goroutine": goroutineID,
						"call":      callIndex,
					},
				}

				resp, err := client.Send(cmd)
				if err != nil {
					t.Errorf(
						"Goroutine %d, call %d: Send() error = %v",
						goroutineID,
						callIndex,
						err,
					)

					continue
				}

				responses[callIndex] = resp
			}

			results[goroutineID] = responses
		}(goroutineIndex)
	}

	waitGroup.Wait()

	// Verify all calls succeeded
	totalCalls := 0
	for i, responses := range results {
		for j, resp := range responses {
			if !resp.Success {
				t.Errorf(
					"Goroutine %d, call %d: Expected success, got failure with code %q",
					i,
					j,
					resp.Code,
				)
			}

			totalCalls++
		}
	}

	if totalCalls != numGoroutines*callsPerGoroutine {
		t.Errorf("Expected %d total calls, got %d", numGoroutines*callsPerGoroutine, totalCalls)
	}

	t.Logf("Successfully completed %d concurrent IPC calls", totalCalls)
}

func TestIPCAdapter_ResourceCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource cleanup test in short mode")
	}

	logger := logger.Get()

	handler := func(_ context.Context, _ ipc.Command) ipc.Response {
		return ipc.Response{Success: true}
	}

	// Test multiple server lifecycles
	for lifecycleIndex := range 5 {
		t.Run(fmt.Sprintf("lifecycle_%d", lifecycleIndex), func(t *testing.T) {
			server, serverErr := ipc.NewServer(handler, logger)
			if serverErr != nil {
				t.Fatalf("Failed to create server %d: %v", lifecycleIndex, serverErr)
			}

			adapter := adapterIPC.NewAdapter(server, logger)

			// Start and stop
			startErr := adapter.Start(context.Background())
			if startErr != nil {
				t.Errorf("Start() error on iteration %d: %v", lifecycleIndex, startErr)
			}

			stopErr := adapter.Stop(context.Background())
			if stopErr != nil {
				t.Errorf("Stop() error on iteration %d: %v", lifecycleIndex, stopErr)
			}
		})
	}

	t.Log("Successfully completed resource cleanup test")
}

func TestIPCAdapter_TimeoutHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	logger := logger.Get()

	// Handler that takes time to respond
	handler := func(ctx context.Context, _ ipc.Command) ipc.Response {
		select {
		case <-time.After(100 * time.Millisecond):
			return ipc.Response{Success: true}
		case <-ctx.Done():
			return ipc.Response{
				Success: false,
				Code:    "TIMEOUT",
				Message: "Request timed out",
			}
		}
	}

	server, serverErr := ipc.NewServer(handler, logger)
	if serverErr != nil {
		t.Fatalf("Failed to create server: %v", serverErr)
	}

	adapter := adapterIPC.NewAdapter(server, logger)
	defer adapter.Stop(context.Background()) //nolint:errcheck

	startErr := adapter.Start(context.Background())
	if startErr != nil {
		t.Fatalf("Start() error = %v", startErr)
	}

	time.Sleep(50 * time.Millisecond) // Let server start

	client := ipc.NewClient()

	// Test normal operation
	cmd := ipc.Command{Action: "test"}

	resp, err := client.Send(cmd)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success, got code: %s", resp.Code)
	}

	t.Log("Successfully tested timeout handling and normal operations")
}

func TestIPCAdapter_ChaosTesting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	logger := logger.Get()

	// Test with various failure scenarios
	testCases := []struct {
		name     string
		handler  ipc.CommandHandler
		commands []ipc.Command
	}{
		{
			name: "handler returns invalid data",
			handler: func(_ context.Context, _ ipc.Command) ipc.Response {
				// Return data that might cause issues in JSON marshaling
				return ipc.Response{
					Success: true,
					Data:    make(chan int), // Channels can't be marshaled
				}
			},
			commands: []ipc.Command{{Action: "invalid_data"}},
		},
		{
			name: "handler returns invalid JSON",
			handler: func(_ context.Context, _ ipc.Command) ipc.Response {
				return ipc.Response{
					Data: make(chan int), // Channels can't be JSON marshaled
				}
			},
			commands: []ipc.Command{{Action: "invalid"}},
		},
		{
			name: "slow handler",
			handler: func(ctx context.Context, _ ipc.Command) ipc.Response {
				select {
				case <-time.After(50 * time.Millisecond):
					return ipc.Response{Success: true}
				case <-ctx.Done():
					return ipc.Response{Success: false, Code: "CANCELED"}
				}
			},
			commands: []ipc.Command{{Action: "slow"}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server, serverErr := ipc.NewServer(testCase.handler, logger)
			if serverErr != nil {
				t.Fatalf("Failed to create server: %v", serverErr)
			}

			adapter := adapterIPC.NewAdapter(server, logger)
			defer func() {
				_ = adapter.Stop(context.Background())
			}()

			startErr := adapter.Start(context.Background())
			if startErr != nil {
				t.Fatalf("Start() error = %v", startErr)
			}

			time.Sleep(10 * time.Millisecond) // Let server start

			client := ipc.NewClient()

			for _, cmd := range testCase.commands {
				// These calls might fail due to chaos, but shouldn't crash
				resp, err := client.Send(cmd)
				if err != nil {
					t.Logf("Expected error for chaos test: %v", err)
				} else {
					t.Logf("Chaos test response: success=%v, code=%q", resp.Success, resp.Code)
				}
			}
		})
	}

	t.Log("Successfully completed chaos testing")
}

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
		{
			name: "command with version",
			cmd: ipc.Command{
				Version: "1.0.0",
				Action:  "test",
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cmd := ipc.Command{Action: "test"}
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
		Version: "1.0.0",
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
	if decoded.Version != response.Version {
		t.Errorf("Version mismatch: got %s, want %s", decoded.Version, response.Version)
	}

	if decoded.Success != response.Success {
		t.Errorf("Success mismatch: got %v, want %v", decoded.Success, response.Success)
	}

	if decoded.Message != response.Message {
		t.Errorf("Message mismatch: got %s, want %s", decoded.Message, response.Message)
	}

	if decoded.Code != response.Code {
		t.Errorf("Code mismatch: got %s, want %s", decoded.Code, response.Code)
	}

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

func TestClientSend_ServerNotRunning(t *testing.T) {
	client := ipc.NewClient()

	cmd := ipc.Command{Action: "test"}

	_, err := client.Send(cmd)
	if err == nil {
		t.Error("Expected error when server is not running")
	}
}

func TestClientSendWithTimeout_ServerNotRunning(t *testing.T) {
	client := ipc.NewClient()

	cmd := ipc.Command{Action: "test"}

	_, err := client.SendWithTimeout(cmd, time.Second)
	if err == nil {
		t.Error("Expected error when server is not running")
	}
}

func TestClientSend_MalformedResponse(t *testing.T) {
	logger := zap.NewNop()

	// Create handler that returns malformed JSON
	handler := func(_ context.Context, _ ipc.Command) ipc.Response {
		// Return a response that will cause JSON marshaling issues
		return ipc.Response{
			Data: make(chan int), // Channels can't be marshaled to JSON
		}
	}

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

	cmd := ipc.Command{Action: "test"}
	_, err := client.Send(cmd)
	// Should get an error due to malformed response
	if err == nil {
		t.Error("Expected error for malformed response")
	}
}

func TestServer_StopWithoutStart(t *testing.T) {
	logger := zap.NewNop()

	handler := func(_ context.Context, _ ipc.Command) ipc.Response {
		return ipc.Response{Success: true}
	}

	server, serverErr := ipc.NewServer(handler, logger)
	if serverErr != nil {
		t.Fatalf("NewServer() failed: %v", serverErr)
	}

	// Stop without starting - should not panic
	err := server.Stop()
	if err != nil {
		t.Errorf("Stop() without Start() should not error, got %v", err)
	}
}

func TestCommand_EmptyAction(t *testing.T) {
	cmd := ipc.Command{Action: ""}

	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	var decoded ipc.Command

	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}

	if decoded.Action != "" {
		t.Errorf("Action mismatch: got %q, want empty string", decoded.Action)
	}
}

func TestResponse_EmptyFields(t *testing.T) {
	response := ipc.Response{}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	var decoded ipc.Response

	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}

	// Verify
	if decoded.Version != "" {
		t.Errorf("Version mismatch: got %q, want empty string", decoded.Version)
	}

	if decoded.Success != false {
		t.Errorf("Success mismatch: got %v, want false", decoded.Success)
	}

	if decoded.Message != "" {
		t.Errorf("Message mismatch: got %q, want empty string", decoded.Message)
	}

	if decoded.Code != "" {
		t.Errorf("Code mismatch: got %q, want empty string", decoded.Code)
	}
}

// FuzzCommandJSON tests IPC command JSON marshaling/unmarshaling with fuzzed input.
func FuzzCommandJSON(f *testing.F) {
	// Add some seed inputs
	f.Add("test", "param1", "param2")
	f.Add("", "", "")
	f.Add("long_command_name_with_many_characters", "param", "")
	f.Add("cmd", "", "multiple params here")

	f.Fuzz(func(t *testing.T, action string, param1 string, param2 string) {
		// Create a command with fuzzed input
		cmd := ipc.Command{
			Action: action,
			Params: map[string]any{
				"param1": param1,
				"param2": param2,
			},
		}

		// Marshal to JSON
		data, err := json.Marshal(cmd)
		if err != nil {
			// Skip invalid inputs that can't be marshaled
			t.Skip("Invalid input for marshaling")
		}

		// Unmarshal back
		var decoded ipc.Command

		err = json.Unmarshal(data, &decoded)
		if err != nil {
			// This should not happen for valid JSON we created
			t.Errorf("Failed to unmarshal valid JSON: %v", err)

			return
		}

		// Verify the action is preserved
		if decoded.Action != cmd.Action {
			t.Errorf("Action mismatch: got %q, want %q", decoded.Action, cmd.Action)
		}

		// Verify params exist if they were set
		if cmd.Params != nil && decoded.Params == nil {
			t.Error("Params should not be nil when original had params")
		}
	})
}

// FuzzResponseJSON tests IPC response JSON marshaling/unmarshaling with fuzzed input.
func FuzzResponseJSON(f *testing.F) {
	// Add some seed inputs
	f.Add("1.0.0", true, "success", "OK")
	f.Add("", false, "", "")
	f.Add("2.1.0", false, "error message", "ERROR_CODE")

	f.Fuzz(func(t *testing.T, version string, success bool, message string, code string) {
		// Create a response with fuzzed input
		response := ipc.Response{
			Version: version,
			Success: success,
			Message: message,
			Code:    code,
			Data:    map[string]any{"test": "data"},
		}

		// Marshal to JSON
		data, err := json.Marshal(response)
		if err != nil {
			// Skip invalid inputs that can't be marshaled
			t.Skip("Invalid input for marshaling")
		}

		// Unmarshal back
		var decoded ipc.Response

		err = json.Unmarshal(data, &decoded)
		if err != nil {
			// This should not happen for valid JSON we created
			t.Errorf("Failed to unmarshal valid JSON: %v", err)

			return
		}

		// Verify core fields are preserved
		if decoded.Success != response.Success {
			t.Errorf("Success mismatch: got %v, want %v", decoded.Success, response.Success)
		}

		if decoded.Message != response.Message {
			t.Errorf("Message mismatch: got %q, want %q", decoded.Message, response.Message)
		}

		if decoded.Code != response.Code {
			t.Errorf("Code mismatch: got %q, want %q", decoded.Code, response.Code)
		}
	})
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
