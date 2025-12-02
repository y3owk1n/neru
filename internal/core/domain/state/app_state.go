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

	// Callbacks
	enabledStateCallback func(bool)

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
		enabled:     true,
		currentMode: domain.ModeIdle,
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
	callback := s.enabledStateCallback
	s.mu.Unlock()

	// Notify callback if state actually changed
	if oldEnabled != enabled && callback != nil {
		// Call callback outside of lock to avoid deadlocks
		go callback(enabled)
	}
}

// OnEnabledStateChanged registers a callback for when the enabled state changes.
func (s *AppState) OnEnabledStateChanged(callback func(bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.enabledStateCallback = callback
	// Call immediately with current state
	if callback != nil {
		go callback(s.enabled)
	}
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
