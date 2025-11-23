package metrics

import (
	"testing"
)

func TestCollector(t *testing.T) {
	c := NewCollector()

	t.Run("IncCounter", func(t *testing.T) {
		c.IncCounter("test_counter", nil)
		snapshot := c.Snapshot()
		if len(snapshot) != 1 {
			t.Errorf("Expected 1 metric, got %d", len(snapshot))
		}
		if snapshot[0].Value != 1.0 {
			t.Errorf("Expected value 1.0, got %f", snapshot[0].Value)
		}
	})

	t.Run("Reset", func(t *testing.T) {
		c.Reset()
		snapshot := c.Snapshot()
		if len(snapshot) != 0 {
			t.Errorf("Expected 0 metrics, got %d", len(snapshot))
		}
	})
}
