package trace_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/trace"
)

func TestTraceID(t *testing.T) {
	t.Run("NewID generates unique IDs", func(t *testing.T) {
		id1 := trace.NewID()
		id2 := trace.NewID()

		if id1 == "" {
			t.Error("NewID returned empty string")
		}

		if id2 == "" {
			t.Error("NewID returned empty string")
		}

		if id1 == id2 {
			t.Error("NewID generated duplicate IDs")
		}
	})

	t.Run("Context propagation", func(t *testing.T) {
		context := context.Background()
		contextID := trace.NewID()

		context = trace.WithTraceID(context, contextID)
		got := trace.FromContext(context)

		if got != contextID {
			t.Errorf("FromContext() = %v, want %v", got, contextID)
		}
	})

	t.Run("FromContext returns empty for missing ID", func(t *testing.T) {
		context := context.Background()
		got := trace.FromContext(context)

		if got != "" {
			t.Errorf("FromContext() = %v, want empty string", got)
		}
	})

	t.Run("String method", func(t *testing.T) {
		id := trace.ID("test-trace-id")
		str := id.String()

		if str != "test-trace-id" {
			t.Errorf("String() = %v, want %v", str, "test-trace-id")
		}
	})
}
