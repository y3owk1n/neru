package hint

import (
	"strings"
	"sync"
	"time"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"go.uber.org/zap"
)

// Manager handles hint generation and management.
type Manager struct {
	domain.BaseManager

	hints            *Collection
	onUpdate         func([]*Interface) // Callback when filtered hints change
	debounceTimer    *time.Timer
	debounceDuration time.Duration
	mu               sync.Mutex // Protects onUpdate callback

	// externalMu is an optional mutex acquired by debouncedUpdate's timer
	// callback before invoking onUpdate. This allows the caller to protect
	// shared state (e.g., screen bounds, overlay manager) that the callback
	// reads/writes without introducing locking inside the callback itself
	// (which would deadlock on synchronous call paths).
	externalMu *sync.Mutex

	// Performance optimization: reuse slice buffer for filtered hints
	cachedFilteredHints []*Interface
}

const (
	// DefaultDebounceDuration is the default debounce duration for hint updates.
	DefaultDebounceDuration = 50 * time.Millisecond
)

// NewManager creates a new hint manager with the specified logger and an
// optional external mutex. When non-nil, debouncedUpdate's timer callback
// acquires externalMu before invoking onUpdate so the caller can protect
// shared state (e.g., screen bounds, overlay manager) without locking
// inside the callback itself (which would deadlock on synchronous paths).
func NewManager(logger *zap.Logger, externalMu *sync.Mutex) *Manager {
	return &Manager{
		BaseManager: domain.BaseManager{
			Logger: logger,
		},
		debounceDuration: DefaultDebounceDuration,
		externalMu:       externalMu,
	}
}

// SetUpdateCallback sets the callback function to be called when filtered hints change.
func (m *Manager) SetUpdateCallback(callback func([]*Interface)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onUpdate = callback
}

// SetHints updates the current hint collection and resets the input state.
func (m *Manager) SetHints(hints *Collection) {
	// Cancel any pending debounced updates
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
		m.debounceTimer = nil
	}

	m.hints = hints
	m.SetCurrentInput("")

	// Trigger immediate update callback with all hints on initial set
	if hints != nil {
		m.mu.Lock()
		callback := m.onUpdate
		m.mu.Unlock()

		if callback != nil {
			callback(hints.All())
		}
	}
}

// Reset clears the current input.
func (m *Manager) Reset() {
	// Cancel any pending debounced updates
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
		m.debounceTimer = nil
	}

	m.SetCurrentInput("")
	// Trigger immediate update callback with all hints
	if m.hints != nil {
		m.mu.Lock()
		callback := m.onUpdate
		m.mu.Unlock()

		if callback != nil {
			callback(m.hints.All())
		}
	}
}

// HandleInput processes an input character and returns the matched hint if an exact match is found.
// Handles backspace for input correction, filters hints by prefix, and detects exact matches.
// Returns (hint, true) if exact match found, (nil, false) otherwise.
// Maintains input state and triggers overlay updates for filtered hints.
func (m *Manager) HandleInput(key string) (*Interface, bool) {
	if m.hints == nil {
		return nil, false
	}

	if m.Logger != nil {
		m.Logger.Debug("Hint manager: Processing input",
			zap.String("key", key),
			zap.String("current_input", m.CurrentInput()))
	}

	// Handle backspace to allow input correction
	if config.IsBackspaceKey(key) {
		if len(m.CurrentInput()) > 0 {
			m.SetCurrentInput(m.CurrentInput()[:len(m.CurrentInput())-1])

			// Update overlay to show filtered hints with new prefix
			if m.hints != nil {
				filtered := m.hints.FilterByPrefix(m.CurrentInput())

				// Reuse cached buffer
				if cap(m.cachedFilteredHints) < len(filtered) {
					m.cachedFilteredHints = make([]*Interface, len(filtered))
				} else {
					m.cachedFilteredHints = m.cachedFilteredHints[:len(filtered)]
				}

				for i, h := range filtered {
					m.cachedFilteredHints[i] = h.WithMatchedPrefix(m.CurrentInput())
				}

				m.debouncedUpdate(m.cachedFilteredHints)
			}
		} else {
			// Reset to show all hints if backspacing from empty input
			m.Reset()
		}

		return nil, false
	}

	// Ignore non-single-character keys
	if len(key) != 1 {
		return nil, false
	}

	// Accumulate input (convert to uppercase to match hints)
	m.SetCurrentInput(m.CurrentInput() + strings.ToUpper(key))

	// Filter hints by prefix
	filtered := m.hints.FilterByPrefix(m.CurrentInput())
	if m.Logger != nil {
		m.Logger.Debug("Hint manager: Filtered hints", zap.Int("filtered_count", len(filtered)))
	}

	if len(filtered) == 0 {
		// No matches - reset and show all hints immediately
		m.SetCurrentInput("")

		if m.hints != nil {
			m.debouncedUpdate(m.hints.All())
		}

		return nil, false
	}

	// Update matched prefix for all filtered hints using cached buffer
	if cap(m.cachedFilteredHints) < len(filtered) {
		m.cachedFilteredHints = make([]*Interface, len(filtered))
	} else {
		m.cachedFilteredHints = m.cachedFilteredHints[:len(filtered)]
	}

	for i, h := range filtered {
		m.cachedFilteredHints[i] = h.WithMatchedPrefix(m.CurrentInput())
	}

	// Check for exact match
	if len(m.cachedFilteredHints) == 1 && m.cachedFilteredHints[0].Label() == m.CurrentInput() {
		if m.Logger != nil {
			m.Logger.Info("Hint manager: Exact match found",
				zap.String("label", m.cachedFilteredHints[0].Label()))
		}

		return m.cachedFilteredHints[0], true
	}

	// Notify update callback with filtered hints (with matched prefix set)
	m.debouncedUpdate(m.cachedFilteredHints)

	return nil, false
}

// FilteredHints returns hints filtered by the current input.
func (m *Manager) FilteredHints() []*Interface {
	if m.hints == nil {
		return nil
	}

	if m.CurrentInput() == "" {
		return m.hints.All()
	}

	return m.hints.FilterByPrefix(m.CurrentInput())
}

// debouncedUpdate schedules a debounced update of the overlay.
func (m *Manager) debouncedUpdate(hints []*Interface) {
	// Cancel any existing timer
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
	}

	// Copy hints to avoid race with slice reuse
	hintsCopy := make([]*Interface, len(hints))
	copy(hintsCopy, hints)

	// Start new timer
	m.debounceTimer = time.AfterFunc(m.debounceDuration, func() {
		// Acquire the external mutex first (if set) so the callback can
		// safely access caller-owned state (e.g., screen bounds, overlay).
		if m.externalMu != nil {
			m.externalMu.Lock()
			defer m.externalMu.Unlock()
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		if m.onUpdate != nil {
			m.onUpdate(hintsCopy)
		}
	})
}
