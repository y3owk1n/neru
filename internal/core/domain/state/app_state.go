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
	enabled              bool
	currentMode          domain.Mode
	hiddenForScreenShare bool

	// Callbacks - stored as map for O(1) unsubscribe
	// Note: Both callback maps share a single nextCallbackID counter to ensure
	// globally unique subscription IDs. Each Off* method only unsubscribes from
	// its corresponding map, so misdirected unsubscribes are safely ignored.
	enabledStateCallbacks     map[uint64]func(bool)
	screenShareStateCallbacks map[uint64]func(bool)
	nextCallbackID            uint64

	// Operational flags
	hotkeysRegistered        bool
	screenChangeProcessing   bool
	screenChangePendingRetry bool
	gridOverlayNeedsRefresh  bool
	hintOverlayNeedsRefresh  bool
	hotkeyRefreshPending     bool
}

// NewAppState creates a new AppState with default values.
func NewAppState() *AppState {
	return &AppState{
		enabled:                   true,
		currentMode:               domain.ModeIdle,
		hiddenForScreenShare:      false,
		enabledStateCallbacks:     make(map[uint64]func(bool)),
		screenShareStateCallbacks: make(map[uint64]func(bool)),
		nextCallbackID:            1, // Start at 1 so 0 can be used as nil sentinel
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

// IsHiddenForScreenShare returns whether the overlay is hidden from screen sharing.
func (s *AppState) IsHiddenForScreenShare() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.hiddenForScreenShare
}

// SetHiddenForScreenShare sets whether the overlay should be hidden from screen sharing.
func (s *AppState) SetHiddenForScreenShare(hidden bool) {
	s.mu.Lock()
	oldHidden := s.hiddenForScreenShare
	s.hiddenForScreenShare = hidden
	// Copy callbacks to slice for iteration outside lock
	callbacks := make([]func(bool), 0, len(s.screenShareStateCallbacks))
	for _, cb := range s.screenShareStateCallbacks {
		callbacks = append(callbacks, cb)
	}

	s.mu.Unlock()

	// Notify all callbacks if state actually changed
	if oldHidden != hidden {
		for _, callback := range callbacks {
			if callback != nil {
				// Call callback outside of lock to avoid deadlocks
				go callback(hidden)
			}
		}
	}
}

// OnScreenShareStateChanged registers a callback for when the screen share state changes.
// Returns a subscription ID that can be used to unsubscribe later.
func (s *AppState) OnScreenShareStateChanged(callback func(bool)) uint64 {
	if callback == nil {
		return 0
	}

	s.mu.Lock()

	nextCallbackID := s.nextCallbackID
	s.nextCallbackID++
	s.screenShareStateCallbacks[nextCallbackID] = callback
	currentState := s.hiddenForScreenShare

	s.mu.Unlock()

	// Fire initial callback via goroutine for consistency with SetHiddenForScreenShare
	go callback(currentState)

	return nextCallbackID
}

// OffScreenShareStateChanged unsubscribes a callback using the ID returned by OnScreenShareStateChanged.
func (s *AppState) OffScreenShareStateChanged(id uint64) {
	s.mu.Lock()
	delete(s.screenShareStateCallbacks, id)
	s.mu.Unlock()
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

// TrySetScreenChangeProcessing atomically sets the processing flag to true only if it's
// currently false. If processing is already in progress, it sets a pending-retry flag so
// the caller knows to re-run after the current processing completes.
// Returns true if the flag was successfully set (caller should proceed), false if
// processing is already in progress (pending-retry flag has been set).
func (s *AppState) TrySetScreenChangeProcessing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.screenChangeProcessing {
		s.screenChangePendingRetry = true

		return false
	}

	s.screenChangeProcessing = true

	return true
}

// FinishScreenChangeProcessing clears the processing flag and returns whether a
// retry was requested while processing was in progress. If true is returned, the
// pending-retry flag is also cleared and the caller should re-process.
func (s *AppState) FinishScreenChangeProcessing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.screenChangeProcessing = false
	retry := s.screenChangePendingRetry
	s.screenChangePendingRetry = false

	return retry
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
