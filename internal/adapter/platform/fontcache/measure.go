package fontcache

import (
	"sync"

	"github.com/y3owk1n/neru/internal/ports"
)

// maxMeasurements bounds what a Measurer remembers. Starting over costs one
// re-measurement per string still in use, which is cheaper than tracking which
// entries are.
const maxMeasurements = 4096

// measureKey is one measurement, named by everything the answer depends on.
type measureKey struct {
	text   string
	family string
	size   float64
	bold   bool
}

// Measurer answers how much room a string takes at a font, remembering each
// answer so the platform's text layer is asked once per string and font. It is
// safe for concurrent use.
//
// What it measures comes from configuration and from the displays attached, so
// a session asks for a few hundred strings. Somebody changing fonts and sizes
// again and again would still add to it for the daemon's lifetime, so it holds
// at most maxMeasurements and starts over when it is full. A failure is not
// remembered, so a text layer that was briefly unusable is asked again.
type Measurer struct {
	mu      sync.RWMutex
	entries map[measureKey]ports.TextMetrics
	measure func(text, family string, size float64, bold bool) (ports.TextMetrics, error)
}

// NewMeasurer returns a Measurer that remembers what measure answers.
func NewMeasurer(
	measure func(text, family string, size float64, bold bool) (ports.TextMetrics, error),
) *Measurer {
	return &Measurer{
		entries: make(map[measureKey]ports.TextMetrics),
		measure: measure,
	}
}

// Measure implements ports.TextMeasurer.
func (m *Measurer) Measure(
	text, family string,
	size float64,
	bold bool,
) (ports.TextMetrics, error) {
	key := measureKey{text: text, family: family, size: size, bold: bold}

	m.mu.RLock()
	cached, ok := m.entries[key]
	m.mu.RUnlock()

	if ok {
		return cached, nil
	}

	metrics, err := m.measure(text, family, size, bold)
	if err != nil {
		return ports.TextMetrics{}, err
	}

	m.mu.Lock()
	if len(m.entries) >= maxMeasurements {
		clear(m.entries)
	}

	m.entries[key] = metrics
	m.mu.Unlock()

	return metrics, nil
}
