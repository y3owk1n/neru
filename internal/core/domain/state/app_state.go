package state

import (
	"sync"

	"github.com/y3owk1n/neru/internal/core/domain"
)

// AppState manages the core application state including enabled status,
// current mode, and various operational flags.
type AppState struct {
	mu sync.RWMutex

	// Core state
	enabled     bool
	currentMode domain.Mode

	// Callbacks - stored as map for O(1) unsubscribe
	enabledStateCallbacks map[uint64]func(bool)
	nextCallbackID        uint64

	// Operational flags
	hotkeysRegistered       bool
	screenChangeProcessing  bool
	gridOverlayNeedsRefresh bool
	hintOverlayNeedsRefresh bool
	hotkeyRefreshPending    bool
}

// NewAppState creates a new AppState with default values.
func NewAppState() *AppState {
	return &AppState{
		enabled:               true,
		currentMode:           domain.ModeIdle,
		enabledStateCallbacks: make(map[uint64]func(bool)),
		nextCallbackID:        1, // Start at 1 so 0 can be used as nil sentinel
	}
}

// IsEnabled returns whether the application is enabled.
func (s *AppState) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.enabled
}

// SetEnabled sets the enabled state of the application.
func (s *AppState) SetEnabled(enabled bool) {
	s.mu.Lock()
	oldEnabled := s.enabled
	s.enabled = enabled
	// Copy callbacks to slice for iteration outside lock
	callbacks := make([]func(bool), 0, len(s.enabledStateCallbacks))
	for _, cb := range s.enabledStateCallbacks {
		callbacks = append(callbacks, cb)
	}

	s.mu.Unlock()

	// Notify all callbacks if state actually changed
	if oldEnabled != enabled {
		for _, callback := range callbacks {
			if callback != nil {
				// Call callback outside of lock to avoid deadlocks
				go callback(enabled)
			}
		}
	}
}

// OnEnabledStateChanged registers a callback for when the enabled state changes.
// Returns a subscription ID that can be used to unsubscribe later.
func (s *AppState) OnEnabledStateChanged(callback func(bool)) uint64 {
	if callback == nil {
		return 0
	}

	s.mu.Lock()

	nextCallbackID := s.nextCallbackID
	s.nextCallbackID++
	s.enabledStateCallbacks[nextCallbackID] = callback
	currentState := s.enabled

	s.mu.Unlock()

	// Fire initial callback via goroutine for consistency with SetEnabled
	go callback(currentState)

	return nextCallbackID
}

// OffEnabledStateChanged unsubscribes a callback using the ID returned by OnEnabledStateChanged.
// If the ID is invalid (already unsubscribed or never existed), this is a no-op.
//
// Note: Due to the snapshot-then-invoke pattern in SetEnabled, a callback may fire
// once after unsubscription if SetEnabled was called before OffEnabledStateChanged.
// Callbacks should be idempotent to handle this gracefully.
func (s *AppState) OffEnabledStateChanged(id uint64) {
	s.mu.Lock()
	delete(s.enabledStateCallbacks, id)
	s.mu.Unlock()
}

// Enable enables the application.
func (s *AppState) Enable() {
	s.SetEnabled(true)
}

// Disable disables the application.
func (s *AppState) Disable() {
	s.SetEnabled(false)
}

// CurrentMode returns the current application mode.
func (s *AppState) CurrentMode() domain.Mode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.currentMode
}

// SetMode sets the current application mode.
func (s *AppState) SetMode(mode domain.Mode) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentMode = mode
}

// HotkeysRegistered returns whether hotkeys are currently registered.
func (s *AppState) HotkeysRegistered() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.hotkeysRegistered
}

// SetHotkeysRegistered sets the hotkeys registered flag.
func (s *AppState) SetHotkeysRegistered(registered bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.hotkeysRegistered = registered
}

// ScreenChangeProcessing returns whether a screen change is being processed.
func (s *AppState) ScreenChangeProcessing() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.screenChangeProcessing
}

// SetScreenChangeProcessing sets the screen change processing flag.
func (s *AppState) SetScreenChangeProcessing(processing bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.screenChangeProcessing = processing
}

// GridOverlayNeedsRefresh returns whether the grid overlay needs refresh.
func (s *AppState) GridOverlayNeedsRefresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.gridOverlayNeedsRefresh
}

// SetGridOverlayNeedsRefresh sets the grid overlay refresh flag.
func (s *AppState) SetGridOverlayNeedsRefresh(needs bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gridOverlayNeedsRefresh = needs
}

// HintOverlayNeedsRefresh returns whether the hint overlay needs refresh.
func (s *AppState) HintOverlayNeedsRefresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.hintOverlayNeedsRefresh
}

// SetHintOverlayNeedsRefresh sets the hint overlay refresh flag.
func (s *AppState) SetHintOverlayNeedsRefresh(needs bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.hintOverlayNeedsRefresh = needs
}

// HotkeyRefreshPending returns whether a hotkey refresh is pending.
func (s *AppState) HotkeyRefreshPending() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.hotkeyRefreshPending
}

// SetHotkeyRefreshPending sets the hotkey refresh pending flag.
func (s *AppState) SetHotkeyRefreshPending(pending bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.hotkeyRefreshPending = pending
}
