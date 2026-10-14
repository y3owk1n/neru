//go:build integration && windows

package ipc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
)

// TestClient_SendWithTimeout_AnswersCommandsOnePastAPowerOfTwo sends commands
// whose encoded length sits on and next to a power of two. On a named pipe
// with no buffer quota, a command one byte past a power of two never reached
// the server, and the exchange ended at the read deadline instead.
func TestClient_SendWithTimeout_AnswersCommandsOnePastAPowerOfTwo(t *testing.T) {
	if ipc.IsServerRunning() {
		t.Skip("a neru daemon already owns the IPC endpoint; skipping")
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

	deadline := time.Now().Add(3 * time.Second)
	for !ipc.IsServerRunning() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	client := ipc.NewClient()

	for _, size := range []int{64, 65, 66, 128, 129, 257, 513, 1025} {
		t.Run(fmt.Sprintf("%d bytes", size), func(t *testing.T) {
			cmd := commandOfSize(t, size)

			// Shorter than the server's read deadline, so a hang fails here
			// rather than waiting it out.
			response, sendErr := client.SendWithTimeout(cmd, 2*time.Second)
			if sendErr != nil {
				t.Fatalf("SendWithTimeout() with a %d byte command error = %v, want nil",
					size, sendErr)
			}

			if !response.Success {
				t.Errorf("SendWithTimeout() with a %d byte command: response.Success = false (%s)",
					size, response.Message)
			}
		})
	}
}

// commandOfSize returns a command whose encoding on the wire, the JSON plus
// the newline the client's encoder adds, is exactly size bytes.
func commandOfSize(t *testing.T, size int) ipc.Command {
	t.Helper()

	build := func(padding int) ipc.Command {
		return ipc.Command{
			Version: ipc.BuildVersion(),
			Action:  testCommandAction,
			Args:    []string{strings.Repeat("z", padding)},
		}
	}

	encoded, err := json.Marshal(build(0))
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	padding := size - (len(encoded) + 1)
	if padding < 0 {
		t.Fatalf("the smallest command is %d bytes, more than %d", len(encoded)+1, size)
	}

	return build(padding)
}
