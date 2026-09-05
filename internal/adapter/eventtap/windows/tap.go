//go:build windows

package windows

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
	winplatform "github.com/y3owk1n/neru/internal/adapter/platform/windows"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

// dispatchChBufferSize bounds how many events the hook thread can queue ahead
// of the dispatcher. The Linux tap uses the same figure.
const dispatchChBufferSize = 256

// dispatchEvent is one item on the dispatcher's queue: a key for the handler,
// or a passthrough notification, which rides the same queue so it lands after
// the keys that preceded it.
type dispatchEvent struct {
	key         string
	passthrough tap.PassthroughCallback
	epoch       uint64
}

// EventTap is a keyboard event interceptor on Windows.
type EventTap struct {
	logger *zap.Logger

	mu                   sync.RWMutex
	callback             tap.Callback
	passthroughCallback  tap.PassthroughCallback
	hotkeys              []string
	stickyModifierToggle bool
	enabled              bool

	// Modifier passthrough state, populated by SetModifierPassthrough and
	// SetInterceptedModifierKeys and read by handleKey. Chords are stored
	// canonicalized via tap.CanonicalChord so the form the hook assembles a
	// chord in matches the form the user wrote a hotkey in.
	passthroughEnabled   bool
	passthroughBlacklist map[string]struct{}
	interceptedChords    map[string]struct{}

	// postedModifiers are the sticky modifiers PostModifierEvent is holding
	// down; the hook reads them into every chord like a physical hold.
	// heldModifiers are the ones the user's hands hold, from the hook's own
	// modifier events, so a modifier both posted and held stays in the chord.
	// A set rather than a count: a held key autorepeats its key-down, and the
	// left and right keys of one modifier are not told apart here.
	postedModifiers map[string]struct{}
	heldModifiers   map[string]struct{}

	hook *winplatform.KeyboardHook

	// dispatchCh carries events from the hook thread to dispatchLoop, the one
	// goroutine that calls the handler. Windows silently removes a
	// WH_KEYBOARD_LL hook whose procedure overruns LowLevelHooksTimeout, and
	// the handler holds the mode lock across a click, a mode exit or a hint
	// refresh, so the hook procedure only classifies the event and enqueues.
	// The channel and loop live for the tap's lifetime, like the Linux tap's;
	// stopDispatch ends them from Destroy, and the channel is never closed,
	// so a key the hook delivers during teardown is dropped rather than sent
	// on a closed channel.
	dispatchCh   chan dispatchEvent
	stopDispatch chan struct{}
	stopOnce     sync.Once
	dispatchWg   sync.WaitGroup

	// dispatchEpoch is stamped on every event as it is queued and bumped on
	// every Disable. dispatchLoop drops an event stamped with an earlier
	// epoch, so a key the hook read while the mode was exiting is not
	// delivered to whatever mode comes next.
	dispatchEpoch atomic.Uint64
}

// NewEventTap creates a new event tap.
func NewEventTap(callback tap.Callback, logger *zap.Logger) *EventTap {
	if logger == nil {
		logger = zap.NewNop()
	}

	eventTap := &EventTap{
		logger:       logger.Named("eventtap"),
		callback:     callback,
		dispatchCh:   make(chan dispatchEvent, dispatchChBufferSize),
		stopDispatch: make(chan struct{}),
	}

	eventTap.dispatchWg.Go(eventTap.dispatchLoop)

	return eventTap
}

// Enable enables the event tap.
func (et *EventTap) Enable() {
	et.mu.Lock()
	if et.enabled {
		et.mu.Unlock()

		return
	}

	et.enabled = true
	et.heldModifiers = nil
	et.mu.Unlock()

	hook, err := winplatform.StartKeyboardHook(et.handleKey)
	if err != nil {
		et.logger.Error("failed to start keyboard hook", zap.Error(err))
		et.mu.Lock()
		et.enabled = false
		et.mu.Unlock()

		return
	}

	et.mu.Lock()
	et.hook = hook
	et.mu.Unlock()
}

// Disable disables the event tap.
func (et *EventTap) Disable() {
	et.mu.Lock()
	if !et.enabled {
		et.mu.Unlock()

		return
	}

	hook := et.hook
	et.hook = nil
	et.enabled = false
	et.heldModifiers = nil
	et.mu.Unlock()

	if hook != nil {
		hook.Stop()
	}

	// Anything still queued was read by the hook before it stopped and belongs
	// to the mode that just exited. The bump covers an event dispatchLoop has
	// already taken off the channel; the drain empties the rest. Disable runs
	// on the dispatcher itself when a key exits the mode, which is why neither
	// step may wait on it.
	et.dispatchEpoch.Add(1)

	for {
		select {
		case <-et.dispatchCh:
		default:
			return
		}
	}
}

// Destroy disables the tap and stops its dispatcher, waiting for a key that
// is being delivered to finish. Safe to call more than once.
func (et *EventTap) Destroy() {
	et.Disable()

	et.stopOnce.Do(func() {
		close(et.stopDispatch)
	})

	et.dispatchWg.Wait()
}

// SetHotkeys sets the hotkeys. These are the global [hotkeys] chords the
// platform backend registered, which handleKey leaves to RegisterHotKey rather
// than dispatching itself.
func (et *EventTap) SetHotkeys(hotkeys []string) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.hotkeys = append([]string(nil), hotkeys...)
}

// SetModifierPassthrough enables or disables modifier passthrough and records
// the blacklist of chords Neru must keep consuming even when they are otherwise
// unbound (the mode layer folds the active mode's own hotkeys into this list).
func (et *EventTap) SetModifierPassthrough(enabled bool, blacklist []string) {
	set := tap.CanonicalChordSet(blacklist)

	et.mu.Lock()
	defer et.mu.Unlock()

	et.passthroughEnabled = enabled
	et.passthroughBlacklist = set
}

// SetInterceptedModifierKeys records the modifier chords the active mode still
// wants Neru to consume while passthrough is enabled.
func (et *EventTap) SetInterceptedModifierKeys(keys []string) {
	set := tap.CanonicalChordSet(keys)

	et.mu.Lock()
	defer et.mu.Unlock()

	et.interceptedChords = set
}

// SetPassthroughCallback sets the passthrough callback.
func (et *EventTap) SetPassthroughCallback(cb tap.PassthroughCallback) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.passthroughCallback = cb
}

// SetStickyModifierToggle enables or disables sticky modifier toggle detection.
func (et *EventTap) SetStickyModifierToggle(enabled bool) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.stickyModifierToggle = enabled
}

// PostModifierEvent simulates a physical modifier key press or release. The
// key goes out tagged as Neru's own, so the hook hands it on without reading
// it as a modifier toggle; nothing has to be remembered here the way the X11
// tap remembers its XTest events.
func (et *EventTap) PostModifierEvent(modifier string, isDown bool) {
	modifier = keyvocab.CanonicalModifier(modifier)
	if modifier == "" {
		return
	}

	err := winplatform.PostModifierKey(modifier, isDown)
	if err != nil {
		et.logger.Warn("Failed to post modifier event", zap.Error(err))

		return
	}

	et.notePostedModifier(modifier, isDown)
}

// SetKeyboardLayout sets the keyboard layout.
func (et *EventTap) SetKeyboardLayout(_ string) bool { return true }

// IsEnabled returns whether the event tap is enabled.
func (et *EventTap) IsEnabled() bool {
	et.mu.RLock()
	defer et.mu.RUnlock()

	return et.enabled
}

// SetHandler sets the key handler.
func (et *EventTap) SetHandler(handler func(key string)) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.callback = handler
}

// EnableWithContext enables the event tap with context.
func (et *EventTap) EnableWithContext(_ context.Context) error {
	et.Enable()

	return nil
}

// DisableWithContext disables the event tap with context.
func (et *EventTap) DisableWithContext(_ context.Context) error {
	et.Disable()

	return nil
}

// IsUinputScrollAvailable returns false on Windows.
func IsUinputScrollAvailable() bool {
	return false
}

// IsWaylandEvdevKeyboardActive returns false on Windows (no evdev / Wayland).
func IsWaylandEvdevKeyboardActive() bool {
	return false
}

// notePostedModifier records a modifier this tap holds down (or released) on
// the handler's behalf.
func (et *EventTap) notePostedModifier(modifier string, isDown bool) {
	et.mu.Lock()
	defer et.mu.Unlock()

	if !isDown {
		delete(et.postedModifiers, modifier)

		return
	}

	if et.postedModifiers == nil {
		et.postedModifiers = make(map[string]struct{})
	}

	et.postedModifiers[modifier] = struct{}{}
}

// noteHeldModifier records a modifier the user pressed or released.
func (et *EventTap) noteHeldModifier(modifier string, isDown bool) {
	et.mu.Lock()
	defer et.mu.Unlock()

	if !isDown {
		delete(et.heldModifiers, modifier)

		return
	}

	if et.heldModifiers == nil {
		et.heldModifiers = make(map[string]struct{})
	}

	et.heldModifiers[modifier] = struct{}{}
}

// physicalChord returns chord without the modifiers only this tap holds, as
// the user's hands pressed it.
func (et *EventTap) physicalChord(chord string) string {
	et.mu.RLock()
	defer et.mu.RUnlock()

	if len(et.postedModifiers) == 0 {
		return chord
	}

	parts := strings.Split(chord, "+")
	kept := parts[:0]

	for i, part := range parts {
		if i < len(parts)-1 {
			mod := keyvocab.CanonicalModifier(part)
			_, posted := et.postedModifiers[mod]
			_, held := et.heldModifiers[mod]

			if posted && !held {
				continue
			}
		}

		kept = append(kept, part)
	}

	return strings.Join(kept, "+")
}

// isRegisteredHotkey reports whether key is one of the chords registered as a
// global hotkey, compared in the normalized form both sides are matched in so a
// binding written "Primary+G" answers for the "Ctrl+G" the hook reads.
func (et *EventTap) isRegisteredHotkey(key string) bool {
	normalized := config.NormalizeKeyForComparison(key)
	if normalized == "" {
		return false
	}

	et.mu.RLock()
	defer et.mu.RUnlock()

	for _, hotkey := range et.hotkeys {
		if config.NormalizeKeyForComparison(hotkey) == normalized {
			return true
		}
	}

	return false
}

func (et *EventTap) handleKey(key string, isUp bool) bool {
	if key == "" {
		return false
	}

	if mod := keyvocab.CanonicalModifier(key); mod != "" {
		et.noteHeldModifier(mod, !isUp)

		if et.stickyToggleEnabled() {
			et.dispatchKey(keyvocab.ModifierToggleEvent(mod, !isUp))
		}

		return false
	}

	if isUp {
		if keyUp := keyvocab.KeyUpEvent(key); keyUp != "" {
			et.dispatchKey(keyUp)
		}

		return false
	}

	normalized := keyvocab.NormalizeKey(key)

	// A chord carrying a system-level modifier (ctrl, alt, cmd) is either the
	// application's or Neru's, never both: handing the event on and dispatching
	// it too would run a mode binding and type into the window behind the
	// overlay at once.
	if config.HasPassthroughModifier(normalized) {
		// A chord RegisterHotKey owns is handed back. That mechanism keeps
		// firing while a mode is active and this hook runs ahead of it, so
		// passing the key on without dispatching leaves exactly one of the two
		// to run the binding. Dispatching as well would run it twice, because
		// the mode handler falls back to the global table for a chord the mode
		// does not bind (internal/app/modes/keymap.go, settledKeymaps),
		// and a double-run of "recursive_grid --toggle" exits the mode and
		// re-enters it. macOS answers the same question the same way, in its own
		// tap's hotkey check (platform/darwin/eventtap_darwin.m).
		//
		// Only the chord the user physically pressed qualifies. A sticky
		// modifier is held by this tap, so with Shift sticky a Ctrl+J press
		// reads as Ctrl+Shift+J; handing that back would fire a Ctrl+Shift+J
		// global hotkey the user never pressed. Sticky modifiers are for the
		// next action, not for hotkeys, so the chord is consumed and dispatched
		// instead, and the handler strips the sticky part before resolving it.
		if et.physicalChord(normalized) == normalized && et.isRegisteredHotkey(normalized) {
			return false
		}

		// An unbound chord the user asked to have passed through reaches the
		// application as the real key event it is. Unlike an evdev grab, a
		// low-level hook forwards or blocks each event on its own, so nothing
		// has to be replayed: the modifiers were forwarded when they went
		// down, and the app reads them off the same keyboard state Neru does.
		if et.shouldPassthroughChord(normalized) {
			et.firePassthroughCallback()

			return false
		}

		et.dispatchKey(normalized)

		return true
	}

	// Non-modifier key-down (single character, Shift+letter, named key like
	// Return/Space/arrows): consume the key so it does not reach the
	// application behind the overlay. This matches macOS behavior where all
	// non-modifier keys are consumed by the event tap.
	et.dispatchKey(normalized)

	return true
}

// shouldPassthroughChord reports whether an unbound modifier chord should be
// passed through to the focused application instead of consumed by Neru. It
// mirrors the macOS event-tap decision: passthrough must be enabled and the
// chord must be neither blacklisted nor in the mode's intercepted set. The
// caller has already established that the chord carries Ctrl/Alt/Cmd;
// shift-only chords stay usable inside modes.
func (et *EventTap) shouldPassthroughChord(chord string) bool {
	et.mu.RLock()
	defer et.mu.RUnlock()

	if !et.passthroughEnabled {
		return false
	}

	canonical := tap.CanonicalChord(chord)

	if _, ok := et.passthroughBlacklist[canonical]; ok {
		return false
	}

	if _, ok := et.interceptedChords[canonical]; ok {
		return false
	}

	return true
}

// firePassthroughCallback queues the registered passthrough callback (if any)
// for the dispatcher, so it runs off the hook thread and after the keys that
// preceded it.
func (et *EventTap) firePassthroughCallback() {
	et.mu.RLock()
	callback := et.passthroughCallback
	et.mu.RUnlock()

	if callback != nil {
		et.enqueue(dispatchEvent{passthrough: callback})
	}
}

// dispatchKey queues a key for the dispatcher. It never blocks: the hook
// procedure calls it on the hook thread, which has to return before Windows
// gives up on it.
func (et *EventTap) dispatchKey(key string) {
	if key != "" {
		et.enqueue(dispatchEvent{key: key})
	}
}

func (et *EventTap) enqueue(event dispatchEvent) {
	event.epoch = et.dispatchEpoch.Load()

	select {
	case <-et.stopDispatch:
	case et.dispatchCh <- event:
	default:
		et.logger.Warn("Dispatch queue full, dropping event")
	}
}

// dispatchLoop delivers queued events to the handler, in order, on this one
// goroutine.
func (et *EventTap) dispatchLoop() {
	for {
		select {
		case <-et.stopDispatch:
			return
		case event := <-et.dispatchCh:
			et.deliver(event)
		}
	}
}

func (et *EventTap) deliver(event dispatchEvent) {
	if et.dispatchEpoch.Load() != event.epoch {
		return
	}

	et.mu.RLock()
	callback := et.callback
	et.mu.RUnlock()

	switch {
	case event.passthrough != nil:
		event.passthrough()
	case callback != nil:
		callback(event.key)
	}
}

func (et *EventTap) stickyToggleEnabled() bool {
	et.mu.RLock()
	defer et.mu.RUnlock()

	return et.stickyModifierToggle
}
