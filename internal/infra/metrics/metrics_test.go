package metrics_test

import (
	"testing"

	metrics "github.com/y3owk1n/neru/internal/infra/metrics"
)

func TestCollector(t *testing.T) {
	collector := metrics.NewCollector()

	t.Run("IncCounter", func(t *testing.T) {
		collector.IncCounter("test_counter", nil)

		snapshot := collector.Snapshot()

		if len(snapshot) != 1 {
			t.Errorf("Expected 1 metric, got %d", len(snapshot))
		}

		if snapshot[0].Value != 1.0 {
			t.Errorf("Expected value 1.0, got %f", snapshot[0].Value)
		}
	})

	t.Run("Reset", func(t *testing.T) {
		collector.Reset()

		snapshot := collector.Snapshot()
		if len(snapshot) != 0 {
			t.Errorf("Expected 0 metrics, got %d", len(snapshot))
		}
	})
}

func TestNoOpCollector(t *testing.T) {
	collector := &metrics.NoOpCollector{}

	t.Run("Operations", func(_ *testing.T) {
		// These should not panic or cause any side effects
		collector.IncCounter("test", nil)
		collector.SetGauge("test", 1.0, nil)
		collector.ObserveHistogram("test", 1.0, nil)
		collector.Reset()
	})

	t.Run("Snapshot", func(t *testing.T) {
		snapshot := collector.Snapshot()
		if snapshot != nil {
			t.Error("Expected nil snapshot from NoOpCollector")
		}
	})
}
