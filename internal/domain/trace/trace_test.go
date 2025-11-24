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
		context := context.Background()
		contextID := NewID()

		context = WithTraceID(context, contextID)
		got := FromContext(context)

		if got != contextID {
			t.Errorf("FromContext() = %v, want %v", got, contextID)
		}
	})

	t.Run("FromContext returns empty for missing ID", func(t *testing.T) {
		context := context.Background()
		got := FromContext(context)

		if got != "" {
			t.Errorf("FromContext() = %v, want empty string", got)
		}
	})
}
