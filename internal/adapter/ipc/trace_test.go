package ipc_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
)

func TestTraceID(t *testing.T) {
	t.Run("NewTraceID generates unique IDs", func(t *testing.T) {
		id1 := ipc.NewTraceID()
		id2 := ipc.NewTraceID()

		if id1 == "" {
			t.Error("NewTraceID returned empty string")
		}

		if id2 == "" {
			t.Error("NewTraceID returned empty string")
		}

		if id1 == id2 {
			t.Error("NewTraceID generated duplicate IDs")
		}
	})

	t.Run("String method", func(t *testing.T) {
		id := ipc.TraceID("test-trace-id")
		str := id.String()

		if str != "test-trace-id" {
			t.Errorf("String() = %v, want %v", str, "test-trace-id")
		}
	})
}
