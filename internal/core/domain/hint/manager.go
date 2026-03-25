package hint

import (
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
)

// Manager handles hint generation and management.
type Manager struct {
	domain.BaseManager

	hints            *Collection
	onUpdate         func([]*Interface) // Callback when filtered hints change
	debounceTimer    *time.Timer
	debounceDuration time.Duration
	mu               sync.Mutex // Protects onUpdate callback and updateGen

	// externalMu is an optional mutex acquired by debouncedUpdate's timer
	// callback before invoking onUpdate. This allows the caller to protect
	// shared state (e.g., screen bounds, overlay manager) that the callback
	// reads/writes without introducing locking inside the callback itself
	// (which would deadlock on synchronous call paths).
	externalMu *sync.Mutex

	// updateGen is a monotonically increasing counter incremented by both
	// immediateUpdate and debouncedUpdate. The debounced timer callback
	// captures the generation at scheduling time and skips the onUpdate
	// call if a newer update has occurred since, preventing a stale
	// goroutine from overwriting a fresher immediate update.
	updateGen uint64

	// Performance optimization: reuse slice buffer for filtered hints
	cachedFilteredHints []*Interface

	// lastFilteredLen caches the filtered hint count from the previous
	// HandleInput call, avoiding a redundant FilterByPrefix to capture
	// the "before" count on each keystroke.
	lastFilteredLen int
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
// The caller MUST hold externalMu (when set) — see assertExternalMuHeld.
func (m *Manager) SetHints(hints *Collection) {
	m.assertExternalMuHeld("SetHints")

	// Cancel any pending debounced updates
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
		m.debounceTimer = nil
	}

	// Bump generation so any already-fired (but blocked) debounce goroutine
	// will see a stale generation and skip its onUpdate call.
	m.mu.Lock()
	m.updateGen++
	m.mu.Unlock()

	m.hints = hints
	m.SetCurrentInput("")

	// Reset cached count to match the full hint set
	if hints != nil {
		m.lastFilteredLen = len(hints.All())
	} else {
		m.lastFilteredLen = 0
	}

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
// The caller MUST hold externalMu (when set) — see assertExternalMuHeld.
func (m *Manager) Reset() {
	m.assertExternalMuHeld("Reset")

	// Cancel any pending debounced updates
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
		m.debounceTimer = nil
	}

	// Bump generation so any already-fired (but blocked) debounce goroutine
	// will see a stale generation and skip its onUpdate call.
	m.mu.Lock()
	m.updateGen++
	m.mu.Unlock()

	m.SetCurrentInput("")

	// Reset cached count to match the full hint set
	if m.hints != nil {
		m.lastFilteredLen = len(m.hints.All())
	} else {
		m.lastFilteredLen = 0
	}

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

	// Ignore non-single-character keys
	if len(key) != 1 {
		return nil, false
	}

	prevLen := m.lastFilteredLen

	// Accumulate input (convert to uppercase to match hints)
	m.SetCurrentInput(m.CurrentInput() + strings.ToUpper(key))

	// Filter hints by prefix
	filtered := m.hints.FilterByPrefix(m.CurrentInput())
	if m.Logger != nil {
		m.Logger.Debug("Hint manager: Filtered hints", zap.Int("filtered_count", len(filtered)))
	}

	if len(filtered) == 0 {
		// No matches - reset input and update to show all hints
		m.SetCurrentInput("")
		allLen := len(m.hints.All())
		m.lastFilteredLen = allLen

		if m.hints != nil {
			// Apply the same count-based heuristic as the other paths:
			// if the hint count didn't change (e.g., repeated invalid
			// keystrokes that keep resetting to the full set), update
			// immediately since only text colors change. Otherwise
			// debounce to batch the structural redraw.
			if allLen == prevLen {
				m.immediateUpdate(m.hints.All())
			} else {
				m.debouncedUpdate(m.hints.All())
			}
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

		// Cancel any pending debounced update so it doesn't fire stale data
		// after the caller acts on the match (e.g., exits hint mode).
		if m.debounceTimer != nil {
			m.debounceTimer.Stop()
			m.debounceTimer = nil
		}

		// Bump generation so any already-fired (but blocked) debounce
		// goroutine will see a stale generation and skip its callback.
		m.mu.Lock()
		m.updateGen++
		m.mu.Unlock()

		m.lastFilteredLen = len(filtered)

		return m.cachedFilteredHints[0], true
	}

	m.lastFilteredLen = len(filtered)

	// When the hint set structure changes (count differs), debounce to avoid
	// excessive redraws. When only the matched prefix changed (same count),
	// update immediately — the overlay only repaints text colors (very cheap).
	//
	// This count-based heuristic is safe because Collection is immutable after
	// creation — the underlying hint set cannot change between keystrokes.
	// Typing can only narrow (or maintain) the set; backspacing can only widen
	// (or maintain) it. If Collection ever gains mutation methods, this
	// assumption must be revisited.
	if len(filtered) == prevLen {
		m.immediateUpdate(m.cachedFilteredHints)
	} else {
		m.debouncedUpdate(m.cachedFilteredHints)
	}

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

// HandleBackspace applies the same input-correction behavior used when
// backspace is pressed during HandleInput.
func (m *Manager) HandleBackspace() {
	if len(m.CurrentInput()) > 0 {
		prevLen := m.lastFilteredLen
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

			m.lastFilteredLen = len(filtered)

			// When the hint set structure changes (count differs), debounce
			// to avoid excessive redraws. When only the matched prefix
			// changed (same count), update immediately — the overlay only
			// needs to repaint text colors which is very cheap.
			if len(filtered) == prevLen {
				m.immediateUpdate(m.cachedFilteredHints)
			} else {
				m.debouncedUpdate(m.cachedFilteredHints)
			}
		}

		return
	}

	// Reset to show all hints if backspacing from empty input
	m.Reset()
}

// assertExternalMuHeld is a debug assertion verifying that the caller holds
// externalMu (when set). Methods that invoke the onUpdate callback
// synchronously (SetHints, Reset, immediateUpdate) must be called while
// the caller holds externalMu to protect shared state (e.g., screen bounds,
// overlay manager) accessed by the callback.
//
// TryLock succeeds only if the mutex is NOT held — meaning the caller
// forgot to lock it, which would cause a data race on shared state.
func (m *Manager) assertExternalMuHeld(caller string) {
	if m.externalMu != nil {
		if m.externalMu.TryLock() {
			m.externalMu.Unlock()
			panic("hint.Manager." + caller + ": caller must hold externalMu")
		}
	}
}

// immediateUpdate invokes the update callback synchronously, canceling any
// pending debounced update. Use this for cheap updates (e.g., prefix color
// changes) where the 50ms debounce delay would feel sluggish.
//
// IMPORTANT: The caller MUST hold externalMu (when set). Unlike debouncedUpdate
// (whose timer goroutine acquires externalMu itself), immediateUpdate runs in
// the caller's goroutine and relies on the caller already holding the lock to
// protect shared state (e.g., screen bounds, overlay manager) accessed by the
// callback. A debug assertion verifies this at runtime.
//
// Unlike debouncedUpdate, this does NOT copy the hints slice because the
// callback executes synchronously in the caller's goroutine — the slice
// is consumed (iterated to build overlay hints) before returning. If the
// callback contract ever changes to store or defer processing of the
// slice, a defensive copy must be added here.
func (m *Manager) immediateUpdate(hints []*Interface) {
	m.assertExternalMuHeld("immediateUpdate")

	// Cancel any pending debounced update so it doesn't fire stale data.
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
		m.debounceTimer = nil
	}

	// Bump generation so any already-fired (but blocked) debounce goroutine
	// will see a stale generation and skip its onUpdate call.
	m.mu.Lock()
	m.updateGen++

	callback := m.onUpdate
	m.mu.Unlock()

	if callback != nil {
		callback(hints)
	}
}

// debouncedUpdate schedules a debounced update of the overlay.
func (m *Manager) debouncedUpdate(hints []*Interface) {
	// Cancel any existing timer
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
	}

	// Bump generation and capture it for the closure. If a newer update
	// (immediate or debounced) occurs before the timer fires, the captured
	// generation will be stale and the callback will be skipped.
	m.mu.Lock()
	m.updateGen++
	gen := m.updateGen
	m.mu.Unlock()

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

		// Read callback and check generation under m.mu, then release
		// before invoking the callback. This matches the pattern used by
		// immediateUpdate, SetHints, and Reset — keeping the callback
		// invocation outside m.mu prevents a deadlock if the callback
		// ever re-enters a m.mu-protected method (e.g., SetUpdateCallback).
		m.mu.Lock()

		// A newer update (immediate or debounced) has occurred since this
		// timer was scheduled — our data is stale, so skip the callback.
		if m.updateGen != gen {
			m.mu.Unlock()

			return
		}

		callback := m.onUpdate
		m.mu.Unlock()

		if callback != nil {
			callback(hintsCopy)
		}
	})
}
