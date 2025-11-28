//go:build integration

package ipc_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"go.uber.org/zap"
)

func BenchmarkClientSend(b *testing.B) {
	logger := zap.NewNop()

	handler := func(_ context.Context, _ ipc.Command) ipc.Response {
		return ipc.Response{Success: true}
	}

	server, _ := ipc.NewServer(handler, logger)

	defer func() {
		_ = server.Stop()
	}()

	server.Start()
	time.Sleep(100 * time.Millisecond)

	client := ipc.NewClient()
	cmd := ipc.Command{Action: "test"}

	b.ResetTimer()

	for b.Loop() {
		_, _ = client.Send(cmd)
	}
}

func BenchmarkJSONMarshal(b *testing.B) {
	cmd := ipc.Command{
		Action: "test",
		Params: map[string]any{
			"key": "value",
		},
	}

	for b.Loop() {
		_, _ = json.Marshal(cmd) //nolint:errchkjson
	}
}
