package hint

import (
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
)

// Manager handles hint generation and management.
type Manager struct {
	domain.BaseManager

	hints    *Collection
	onUpdate func([]*Interface) // Callback when filtered hints change
	mu       sync.Mutex         // Protects onUpdate

	// externalMu is an optional mutex the caller holds around every method
	// that invokes onUpdate. The callback runs synchronously in the caller's
	// goroutine, so it can read and write the caller's shared state (e.g.,
	// screen bounds, overlay manager) without locking inside the callback.
	externalMu *sync.Mutex

	// Performance optimization: reuse slice buffer for filtered hints
	cachedFilteredHints []*Interface

	// exitOnUnmatched keeps the input and skips the redraw on an unmatched
	// key, so the mode can exit without drawing first.
	exitOnUnmatched bool
}

// NewManager creates a new hint manager with the specified logger and an
// optional external mutex. When non-nil, every method that invokes onUpdate
// requires the caller to hold externalMu (see requireExternalMuHeld).
func NewManager(logger *zap.Logger, externalMu *sync.Mutex) *Manager {
	return &Manager{
		BaseManager: domain.BaseManager{
			Logger: logger,
		},
		externalMu: externalMu,
	}
}

// SetUpdateCallback sets the callback function to be called when filtered hints change.
func (m *Manager) SetUpdateCallback(callback func([]*Interface)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onUpdate = callback
}

// SetExitOnUnmatched sets whether HandleInput reports an unmatched key
// instead of resetting the input.
func (m *Manager) SetExitOnUnmatched(exit bool) {
	m.exitOnUnmatched = exit
}

// SetHints updates the current hint collection and resets the input state.
// The caller MUST hold externalMu (when set). See requireExternalMuHeld.
func (m *Manager) SetHints(hints *Collection) error {
	err := m.requireExternalMuHeld("SetHints")
	if err != nil {
		return err
	}

	m.hints = hints
	m.SetCurrentInput("")

	if hints == nil {
		return nil
	}

	m.mu.Lock()
	callback := m.onUpdate
	m.mu.Unlock()

	if callback != nil {
		callback(hints.All())
	}

	return nil
}

// Reset clears the current input.
// The caller MUST hold externalMu (when set). See requireExternalMuHeld.
func (m *Manager) Reset() error {
	err := m.requireExternalMuHeld("Reset")
	if err != nil {
		return err
	}

	m.SetCurrentInput("")

	if m.hints == nil {
		return nil
	}

	m.mu.Lock()
	callback := m.onUpdate
	m.mu.Unlock()

	if callback != nil {
		callback(m.hints.All())
	}

	return nil
}

// Clear releases session-scoped hint state without invoking the update
// callback. It is intended for mode teardown, where the overlay has already
// been cleared by the caller and retaining the previous collection would keep
// every generated hint and domain element alive until the next activation.
func (m *Manager) Clear() error {
	err := m.requireExternalMuHeld("Clear")
	if err != nil {
		return err
	}

	m.hints = nil
	m.cachedFilteredHints = nil
	m.exitOnUnmatched = false
	m.SetCurrentInput("")

	return nil
}

// HandleInput processes an input character and returns the matched hint if an exact match is found.
// It filters hints by prefix and detects exact matches.
// It returns the hint and exactMatch=true on an exact match, and unmatched=true
// when no hint matches the input and exitOnUnmatched is enabled.
// Maintains input state and triggers overlay updates for filtered hints.
func (m *Manager) HandleInput(key string) (*Interface, bool, bool, error) {
	if m.hints == nil {
		return nil, false, false, nil
	}

	if len(key) != 1 {
		return nil, false, false, nil
	}

	m.SetCurrentInput(m.CurrentInput() + strings.ToUpper(key))

	filtered := m.hints.FilterByPrefix(m.CurrentInput())
	if m.Logger != nil {
		m.Logger.Debug("Hints filtered by input", zap.Int("filtered_count", len(filtered)))
	}

	if len(filtered) == 0 {
		if m.exitOnUnmatched {
			return nil, false, true, nil
		}

		// No matches, so reset the input and show all hints.
		m.SetCurrentInput("")

		err := m.immediateUpdate(m.hints.All())
		if err != nil {
			return nil, false, false, err
		}

		return nil, false, false, nil
	}

	if cap(m.cachedFilteredHints) < len(filtered) {
		m.cachedFilteredHints = make([]*Interface, len(filtered))
	} else {
		m.cachedFilteredHints = m.cachedFilteredHints[:len(filtered)]
	}

	for i, h := range filtered {
		m.cachedFilteredHints[i] = h.WithMatchedPrefix(m.CurrentInput())
	}

	if len(m.cachedFilteredHints) == 1 && m.cachedFilteredHints[0].Label() == m.CurrentInput() {
		if m.Logger != nil {
			m.Logger.Debug("Hint exact match found")
		}

		return m.cachedFilteredHints[0], true, false, nil
	}

	err := m.immediateUpdate(m.cachedFilteredHints)
	if err != nil {
		return nil, false, false, err
	}

	return nil, false, false, nil
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

// HandleBackspace removes the last input character and redraws the hints it
// still matches. On empty input it resets to the full hint set.
func (m *Manager) HandleBackspace() error {
	if len(m.CurrentInput()) > 0 {
		m.SetCurrentInput(m.CurrentInput()[:len(m.CurrentInput())-1])

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

			err := m.immediateUpdate(m.cachedFilteredHints)
			if err != nil {
				return err
			}
		}

		return nil
	}

	// Reset to show all hints if backspacing from empty input
	return m.Reset()
}

// requireExternalMuHeld verifies that the caller holds externalMu (when set).
// Methods that invoke the onUpdate callback synchronously (SetHints, Reset,
// immediateUpdate) must be called while the caller holds externalMu to protect
// shared state (e.g., screen bounds, overlay manager) accessed by the callback.
//
// TryLock succeeds only if the mutex is NOT held, which means the caller forgot
// to lock it and would race on shared state.
func (m *Manager) requireExternalMuHeld(caller string) error {
	if m.externalMu == nil {
		return nil
	}

	if m.externalMu.TryLock() {
		m.externalMu.Unlock()

		return fmt.Errorf("hint.Manager.%s: %w", caller, ErrExternalMuNotHeld)
	}

	return nil
}

// immediateUpdate runs the update callback synchronously, so a keystroke's
// redraw reaches the overlay before HandleInput returns. The caller must hold
// externalMu when set, because this runs in the caller's goroutine. There is
// no defensive copy of hints because the callback consumes the slice before
// returning. Add one if that contract ever changes.
func (m *Manager) immediateUpdate(hints []*Interface) error {
	err := m.requireExternalMuHeld("immediateUpdate")
	if err != nil {
		return err
	}

	m.mu.Lock()
	callback := m.onUpdate
	m.mu.Unlock()

	if callback != nil {
		callback(hints)
	}

	return nil
}
