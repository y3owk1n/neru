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

	t.Run("SetGauge", func(t *testing.T) {
		collector.SetGauge("test_gauge", 42.5, map[string]string{"unit": "percent"})

		snapshot := collector.Snapshot()

		if len(snapshot) != 2 {
			t.Errorf("Expected 2 metrics, got %d", len(snapshot))
		}

		// Find the gauge metric
		var gaugeMetric *metrics.Metric
		for i := range snapshot {
			if snapshot[i].Name == "test_gauge" {
				gaugeMetric = &snapshot[i]

				break
			}
		}

		if gaugeMetric == nil {
			t.Error("Expected to find test_gauge metric")

			return
		}

		if gaugeMetric.Value != 42.5 {
			t.Errorf("Expected gauge value 42.5, got %f", gaugeMetric.Value)
		}

		if gaugeMetric.Labels["unit"] != "percent" {
			t.Errorf("Expected label unit=percent, got %v", gaugeMetric.Labels)
		}
	})

	t.Run("ObserveHistogram", func(t *testing.T) {
		collector.ObserveHistogram("test_histogram", 0.123, map[string]string{"method": "GET"})

		snapshot := collector.Snapshot()

		if len(snapshot) != 3 {
			t.Errorf("Expected 3 metrics, got %d", len(snapshot))
		}

		// Find the histogram metric
		var histMetric *metrics.Metric
		for i := range snapshot {
			if snapshot[i].Name == "test_histogram" {
				histMetric = &snapshot[i]

				break
			}
		}

		if histMetric == nil {
			t.Error("Expected to find test_histogram metric")

			return
		}

		if histMetric.Value != 0.123 {
			t.Errorf("Expected histogram value 0.123, got %f", histMetric.Value)
		}

		if histMetric.Labels["method"] != "GET" {
			t.Errorf("Expected label method=GET, got %v", histMetric.Labels)
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
