package metrics

import (
	"sync"
	"time"
)

// MetricType represents the type of metric.
type MetricType int

const (
	TypeCounter MetricType = iota
	TypeGauge
	TypeHistogram
)

// Metric represents a single metric data point.
type Metric struct {
	Name      string
	Type      MetricType
	Value     float64
	Labels    map[string]string
	Timestamp time.Time
}

// Collector manages metric collection.
type Collector struct {
	mu      sync.RWMutex
	metrics []Metric
}

// NewCollector creates a new metrics collector.
func NewCollector() *Collector {
	return &Collector{
		metrics: make([]Metric, 0, 1000),
	}
}

// IncCounter increments a counter metric.
func (c *Collector) IncCounter(name string, labels map[string]string) {
	c.addMetric(name, TypeCounter, 1.0, labels)
}

// SetGauge sets a gauge metric.
func (c *Collector) SetGauge(name string, value float64, labels map[string]string) {
	c.addMetric(name, TypeGauge, value, labels)
}

// ObserveHistogram records a histogram observation (e.g., duration).
func (c *Collector) ObserveHistogram(name string, value float64, labels map[string]string) {
	c.addMetric(name, TypeHistogram, value, labels)
}

func (c *Collector) addMetric(
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

// Reset clears all collected metrics.
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics = c.metrics[:0]
}

// Snapshot returns a copy of current metrics.
func (c *Collector) Snapshot() []Metric {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshot := make([]Metric, len(c.metrics))
	copy(snapshot, c.metrics)
	return snapshot
}
