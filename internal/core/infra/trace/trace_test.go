//go:build unit

package trace_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/trace"
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
		ctx := context.Background()
		ctxID := trace.NewID()

		ctx = trace.WithTraceID(ctx, ctxID)
		got := trace.FromContext(ctx)

		if got != ctxID {
			t.Errorf("Fromctx() = %v, want %v", got, ctxID)
		}
	})

	t.Run("Fromctx returns empty for missing ID", func(t *testing.T) {
		ctx := context.Background()
		got := trace.FromContext(ctx)

		if got != "" {
			t.Errorf("Fromctx() = %v, want empty string", got)
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
