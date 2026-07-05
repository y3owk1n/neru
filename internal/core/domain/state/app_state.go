package state

import (
	"sync"

	"github.com/y3owk1n/neru/internal/core/domain"
)

// ModeExitReason indicates how the most recent mode was exited.
type ModeExitReason int

const (
	// ModeExitReasonNone is the initial state — no exit has been observed.
	ModeExitReasonNone ModeExitReason = iota
	// ModeExitReasonCompleted means the user made a deliberate selection.
	ModeExitReasonCompleted
	// ModeExitReasonCancelled means the user dismissed the mode without selecting.
	ModeExitReasonCancelled
)

// AppState manages the core application state including enabled status,
// current mode, and various operational flags.
type AppState struct {
	mu sync.RWMutex

	// Core state
	enabled              bool
	currentMode          domain.Mode
	hiddenForScreenShare bool
	scrollInverted       bool
	modeExitReason       ModeExitReason
	modeExitReasonValid  bool

	// Callbacks - stored as map for O(1) unsubscribe
	// Note: All callback maps share a single nextCallbackID counter to ensure
	// globally unique subscription IDs. Each Off* method only unsubscribes from
	// its corresponding map, so misdirected unsubscribes are safely ignored.
	enabledStateCallbacks      map[uint64]func(bool)
	screenShareStateCallbacks  map[uint64]func(bool)
	scrollInvertStateCallbacks map[uint64]func(bool)
	nextCallbackID             uint64

	// Operational flags
	hotkeysRegistered                bool
	screenChangeProcessing           bool
	screenChangePendingRetry         bool
	gridOverlayNeedsRefresh          bool
	hintOverlayNeedsRefresh          bool
	recursiveGridOverlayNeedsRefresh bool
	hotkeyRefreshPending             bool
}

// NewAppState creates a new AppState with default values.
func NewAppState() *AppState {
	return &AppState{
		enabled:                    true,
		currentMode:                domain.ModeIdle,
		hiddenForScreenShare:       false,
		scrollInverted:             false,
		enabledStateCallbacks:      make(map[uint64]func(bool)),
		screenShareStateCallbacks:  make(map[uint64]func(bool)),
		scrollInvertStateCallbacks: make(map[uint64]func(bool)),
		nextCallbackID:             1, // Start at 1 so 0 can be used as nil sentinel
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

// ToggleEnabled atomically toggles the enabled state and notifies callbacks.
// This avoids the check-then-act race of calling IsEnabled() + SetEnabled().
func (s *AppState) ToggleEnabled() {
	s.mu.Lock()
	oldEnabled := s.enabled
	s.enabled = !oldEnabled
	newEnabled := s.enabled

	// Copy callbacks to slice for iteration outside lock
	callbacks := make([]func(bool), 0, len(s.enabledStateCallbacks))
	for _, cb := range s.enabledStateCallbacks {
		callbacks = append(callbacks, cb)
	}

	s.mu.Unlock()

	// State always changes in a toggle, notify all callbacks
	for _, callback := range callbacks {
		if callback != nil {
			go callback(newEnabled)
		}
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

// ToggleHiddenForScreenShare atomically toggles the screen share hidden state and notifies callbacks.
// This avoids the check-then-act race of calling IsHiddenForScreenShare() + SetHiddenForScreenShare().
func (s *AppState) ToggleHiddenForScreenShare() bool {
	s.mu.Lock()
	oldHidden := s.hiddenForScreenShare
	s.hiddenForScreenShare = !oldHidden
	newHidden := s.hiddenForScreenShare

	// Copy callbacks to slice for iteration outside lock
	callbacks := make([]func(bool), 0, len(s.screenShareStateCallbacks))
	for _, cb := range s.screenShareStateCallbacks {
		callbacks = append(callbacks, cb)
	}

	s.mu.Unlock()

	// State always changes in a toggle, notify all callbacks
	for _, callback := range callbacks {
		if callback != nil {
			go callback(newHidden)
		}
	}

	return newHidden
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

// IsScrollInverted returns whether scroll direction inversion is enabled.
func (s *AppState) IsScrollInverted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.scrollInverted
}

// SetScrollInverted sets whether scroll direction inversion is enabled.
func (s *AppState) SetScrollInverted(inverted bool) {
	s.mu.Lock()
	oldInverted := s.scrollInverted
	s.scrollInverted = inverted

	callbacks := make([]func(bool), 0, len(s.scrollInvertStateCallbacks))
	for _, cb := range s.scrollInvertStateCallbacks {
		callbacks = append(callbacks, cb)
	}

	s.mu.Unlock()

	if oldInverted != inverted {
		for _, callback := range callbacks {
			if callback != nil {
				go callback(inverted)
			}
		}
	}
}

// ToggleScrollInverted atomically toggles the scroll invert state and notifies callbacks.
// This avoids the check-then-act race of calling IsScrollInverted() + SetScrollInverted().
func (s *AppState) ToggleScrollInverted() bool {
	s.mu.Lock()
	oldInverted := s.scrollInverted
	s.scrollInverted = !oldInverted
	newInverted := s.scrollInverted

	// Copy callbacks to slice for iteration outside lock
	callbacks := make([]func(bool), 0, len(s.scrollInvertStateCallbacks))
	for _, cb := range s.scrollInvertStateCallbacks {
		callbacks = append(callbacks, cb)
	}

	s.mu.Unlock()

	// State always changes in a toggle, notify all callbacks
	for _, callback := range callbacks {
		if callback != nil {
			go callback(newInverted)
		}
	}

	return newInverted
}

// OnScrollInvertStateChanged registers a callback for when the scroll invert state changes.
// Returns a subscription ID that can be used to unsubscribe later.
func (s *AppState) OnScrollInvertStateChanged(callback func(bool)) uint64 {
	if callback == nil {
		return 0
	}

	s.mu.Lock()

	nextCallbackID := s.nextCallbackID
	s.nextCallbackID++
	s.scrollInvertStateCallbacks[nextCallbackID] = callback
	currentState := s.scrollInverted

	s.mu.Unlock()

	// Fire initial callback via goroutine for consistency with SetScrollInverted
	go callback(currentState)

	return nextCallbackID
}

// OffScrollInvertStateChanged unsubscribes a callback using the ID returned by OnScrollInvertStateChanged.
func (s *AppState) OffScrollInvertStateChanged(id uint64) {
	s.mu.Lock()
	delete(s.scrollInvertStateCallbacks, id)
	s.mu.Unlock()
}

// CurrentMode returns the current application mode.
func (s *AppState) CurrentMode() domain.Mode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.currentMode
}

// SetMode sets the current application mode.
// When entering a non-idle mode, any stale exit reason is invalidated.
func (s *AppState) SetMode(mode domain.Mode) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if mode != domain.ModeIdle {
		s.modeExitReason = ModeExitReasonNone
		s.modeExitReasonValid = false
	}

	s.currentMode = mode
}

// SetModeExitReason records how the current mode session exited. Must be
// called before the mode transitions to idle (before exitModeLocked).
func (s *AppState) SetModeExitReason(reason ModeExitReason) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.modeExitReason = reason
	s.modeExitReasonValid = true
}

// ConsumeModeExitReason atomically reads and resets the exit reason.
// Returns ModeExitReasonNone if no valid reason was recorded.
// This ensures each consumer (e.g. wait_for_mode_exit --bail) sees the
// value exactly once and stale values don't leak between chains.
func (s *AppState) ConsumeModeExitReason() ModeExitReason {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.modeExitReasonValid {
		return ModeExitReasonNone
	}

	reason := s.modeExitReason
	s.modeExitReason = ModeExitReasonNone
	s.modeExitReasonValid = false

	return reason
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

// ResetScreenChangeProcessing unconditionally clears both the processing
// and pending-retry flags. It is intended only for panic-recovery paths
// where the normal Finish protocol cannot complete.
func (s *AppState) ResetScreenChangeProcessing() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.screenChangeProcessing = false
	s.screenChangePendingRetry = false
}

// FinishScreenChangeProcessing checks whether a retry was requested while
// processing was in progress. If a retry is pending, the processing flag
// remains set (caller retains exclusive ownership) and the pending-retry
// flag is cleared — the caller should re-process. If no retry is pending,
// the processing flag is cleared and the caller is done.
func (s *AppState) FinishScreenChangeProcessing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	retry := s.screenChangePendingRetry
	s.screenChangePendingRetry = false

	if !retry {
		s.screenChangeProcessing = false
	}

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

// RecursiveGridOverlayNeedsRefresh returns whether the recursive-grid overlay needs refresh.
func (s *AppState) RecursiveGridOverlayNeedsRefresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.recursiveGridOverlayNeedsRefresh
}

// SetRecursiveGridOverlayNeedsRefresh sets the recursive-grid overlay refresh flag.
func (s *AppState) SetRecursiveGridOverlayNeedsRefresh(needs bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.recursiveGridOverlayNeedsRefresh = needs
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
