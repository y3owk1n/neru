package trace

import (
	"context"
	"testing"
)

func TestTraceID(t *testing.T) {
	t.Run("NewID generates unique IDs", func(t *testing.T) {
		id1 := NewID()
		id2 := NewID()

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
		id := NewID()

		ctx = WithTraceID(ctx, id)
		got := FromContext(ctx)

		if got != id {
			t.Errorf("FromContext() = %v, want %v", got, id)
		}
	})

	t.Run("FromContext returns empty for missing ID", func(t *testing.T) {
		ctx := context.Background()
		got := FromContext(ctx)

		if got != "" {
			t.Errorf("FromContext() = %v, want empty string", got)
		}
	})
}
