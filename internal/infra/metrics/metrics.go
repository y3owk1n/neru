package metrics

import (
	"sync"
	"time"
)

// MetricType defines the type of metric.
type MetricType int

const (
	// TypeCounter represents a counter metric.
	TypeCounter MetricType = iota
	// TypeGauge represents a gauge metric.
	TypeGauge
	// TypeHistogram represents a histogram metric.
	TypeHistogram
)

const (
	// DefaultMetricsCapacity is the default capacity for metrics.
	DefaultMetricsCapacity = 1000
)

// Metric represents a single metric data point.
type Metric struct {
	Name      string            `json:"name"`
	Type      MetricType        `json:"type"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels"`
	Timestamp time.Time         `json:"timestamp"`
}

// StandardCollector manages metric collection.
type StandardCollector struct {
	mu      sync.RWMutex
	metrics []Metric
}

// Collector defines the interface for metrics collection.
type Collector interface {
	IncCounter(name string, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
	ObserveHistogram(name string, value float64, labels map[string]string)
	Reset()
	Snapshot() []Metric
}

// NoOpCollector is a no-op implementation of Collector.
// Used when metrics collection is disabled.
type NoOpCollector struct{}

// IncCounter is a no-op implementation.
func (c *NoOpCollector) IncCounter(_ string, _ map[string]string) {}

// SetGauge is a no-op implementation.
func (c *NoOpCollector) SetGauge(_ string, _ float64, _ map[string]string) {}

// ObserveHistogram is a no-op implementation.
func (c *NoOpCollector) ObserveHistogram(_ string, _ float64, _ map[string]string) {}

// Reset is a no-op implementation.
func (c *NoOpCollector) Reset() {}

// Snapshot returns nil for no-op collector.
func (c *NoOpCollector) Snapshot() []Metric { return nil }

// NewCollector creates a new metrics collector.
func NewCollector() *StandardCollector {
	return &StandardCollector{
		metrics: make([]Metric, 0, DefaultMetricsCapacity),
	}
}

// IncCounter increments a counter metric.
func (c *StandardCollector) IncCounter(name string, labels map[string]string) {
	c.addMetric(name, TypeCounter, 1.0, labels)
}

// SetGauge sets a gauge metric.
func (c *StandardCollector) SetGauge(name string, value float64, labels map[string]string) {
	c.addMetric(name, TypeGauge, value, labels)
}

// ObserveHistogram records a histogram observation (e.g., duration).
func (c *StandardCollector) ObserveHistogram(name string, value float64, labels map[string]string) {
	c.addMetric(name, TypeHistogram, value, labels)
}

// Reset clears all collected metrics.
func (c *StandardCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics = c.metrics[:0]
}

// Snapshot returns a copy of current metrics.
func (c *StandardCollector) Snapshot() []Metric {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshot := make([]Metric, len(c.metrics))
	copy(snapshot, c.metrics)

	return snapshot
}

func (c *StandardCollector) addMetric(
	name string,
	typ MetricType,
	value float64,
	labels map[string]string,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics = append(c.metrics, Metric{
		Name:      name,
		Type:      typ,
		Value:     value,
		Labels:    labels,
		Timestamp: time.Now(),
	})
}
