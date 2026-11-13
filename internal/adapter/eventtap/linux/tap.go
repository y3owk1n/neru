//go:build linux

package linux

import (
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

type pendingSyntheticModifierEvent struct {
	modifier  string
	isDown    bool
	expiresAt time.Time
}

const syntheticModifierSuppressionWindow = 250 * time.Millisecond

const dispatchChBufferSize = 256

// linuxModifierState tracks the reference count of each modifier key group.
// Counts can go negative when a grab starts while modifiers are held (the
// release arrives without its press), which allZero's <= 0 check tolerates.
type linuxModifierState struct {
	shift int
	ctrl  int
	alt   int
	cmd   int
}

func (s *linuxModifierState) update(modifier string, isDown bool) {
	delta := 1
	if !isDown {
		delta = -1
	}

	switch modifier {
	case evdevModifierShift:
		s.shift += delta
	case evdevModifierCtrl:
		s.ctrl += delta
	case evdevModifierAlt:
		s.alt += delta
	case evdevModifierCmd:
		s.cmd += delta
	}
}

func (s *linuxModifierState) allZero() bool {
	return s.shift <= 0 && s.ctrl <= 0 && s.alt <= 0 && s.cmd <= 0
}

// EventTap intercepts keyboard events on Linux.
type EventTap struct {
	logger *zap.Logger

	mu                   sync.RWMutex
	callback             tap.Callback
	passthroughCallback  tap.PassthroughCallback
	hotkeys              []string
	stickyModifierToggle bool
	enabled              bool

	// Modifier passthrough state, set by SetModifierPassthrough and
	// SetInterceptedModifierKeys. Only the Wayland evdev backend reads it
	// (evdevSession.handlePress), because only the proxy keyboard can re-emit
	// an unbound chord. The X11 grab and the wl-keyboard fallback cannot.
	// Chords are canonicalized via tap.CanonicalChord.
	passthroughEnabled   bool
	passthroughBlacklist map[string]struct{}
	interceptedChords    map[string]struct{}

	// Sticky modifier events dispatch only once every initially held
	// modifier is released, as on macOS. SetStickyModifierToggle(true)
	// disarms, and the platform handler re-arms on a clean slate.
	stickyModifierDetectionArmed bool

	syntheticModifierEvents []pendingSyntheticModifierEvent

	stopCh chan struct{}
	doneCh chan struct{}

	// dispatchCh decouples the event-tap goroutine from the callback, as on
	// macOS, so a key whose dispatch exits the mode cannot deadlock waiting
	// for the event-tap goroutine to stop.
	dispatchCh chan string
	dispatchWg sync.WaitGroup
	destroyed  bool

	// dispatchEpoch is bumped on every Disable. dispatchLoop checks it is
	// unchanged before invoking the callback, so stale buffered events never
	// leak across enable/disable cycles.
	dispatchEpoch atomic.Uint64

	// overlayPassiveOnce gates telling the Wayland overlay to stop asking for
	// keyboard focus once the evdev proxy owns the keys.
	overlayPassiveOnce sync.Once

	// dropWarned limits a full dispatch channel to one warning until it drains.
	dropWarned atomic.Bool

	// x11Grabbed is true from the X11 tap's grab until its run ends,
	// through a yield, and yieldCh is how YieldKeyboard asks it to step aside.
	x11Grabbed atomic.Bool
	yieldCh    chan x11Yield

	// overlayCapture is true while the Wayland fallback holds the keyboard
	// through Neru's own overlay surface.
	overlayCapture atomic.Bool
}

// NewEventTap creates a new EventTap.
func NewEventTap(callback tap.Callback, logger *zap.Logger) *EventTap {
	tap := &EventTap{
		logger:   logger,
		callback: callback,
	}
	tap.dispatchCh = make(chan string, dispatchChBufferSize)
	tap.yieldCh = make(chan x11Yield)
	tap.dispatchWg.Add(1)

	go tap.dispatchLoop()

	return tap
}

// Enable starts intercepting keyboard events.
func (et *EventTap) Enable() {
	et.mu.Lock()
	if et.enabled || et.destroyed {
		et.mu.Unlock()

		return
	}

	et.stopCh = make(chan struct{})
	et.doneCh = make(chan struct{})
	et.enabled = true
	et.mu.Unlock()

	go et.run()
}

// Disable stops intercepting keyboard events.
func (et *EventTap) Disable() {
	et.mu.Lock()
	if !et.enabled {
		et.mu.Unlock()

		return
	}

	stopCh := et.stopCh
	doneCh := et.doneCh
	et.enabled = false
	et.mu.Unlock()

	close(stopCh)

	<-doneCh

	// Discard any event dispatchLoop picked up before the drain.
	et.dispatchEpoch.Add(1)

	// Drain stale events. The evdev goroutine has exited, so whatever is
	// buffered predates the stop and must not reach the next mode's handler.
	for {
		select {
		case <-et.dispatchCh:
		default:
			return
		}
	}
}

// Destroy stops and cleans up the EventTap. Later calls are no-ops.
func (et *EventTap) Destroy() {
	et.mu.Lock()
	if et.destroyed {
		et.mu.Unlock()

		return
	}

	et.destroyed = true
	et.mu.Unlock()

	et.Disable()

	// dispatchCh lives as long as the EventTap, so closing it is what stops
	// the dispatch goroutine.
	close(et.dispatchCh)
	et.dispatchWg.Wait()
}

// SetHandler sets the callback for key events.
func (et *EventTap) SetHandler(handler func(key string)) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.callback = handler
}

// SetHotkeys configures the hotkey list.
func (et *EventTap) SetHotkeys(hotkeys []string) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.hotkeys = append([]string(nil), hotkeys...)
}

// SetModifierPassthrough enables or disables modifier passthrough and records
// the chords Neru must keep consuming even when unbound (the mode layer folds
// the active mode's hotkeys in). Only the Wayland evdev backend acts on it.
func (et *EventTap) SetModifierPassthrough(enabled bool, blacklist []string) {
	set := tap.CanonicalChordSet(blacklist)

	et.mu.Lock()
	et.passthroughEnabled = enabled
	et.passthroughBlacklist = set
	et.mu.Unlock()
}

// SetInterceptedModifierKeys records the modifier chords the active mode still
// wants Neru to consume while passthrough is enabled.
func (et *EventTap) SetInterceptedModifierKeys(keys []string) {
	set := tap.CanonicalChordSet(keys)

	et.mu.Lock()
	et.interceptedChords = set
	et.mu.Unlock()
}

// SetPassthroughCallback sets the callback for passthrough mode.
func (et *EventTap) SetPassthroughCallback(cb tap.PassthroughCallback) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.passthroughCallback = cb
}

// SetStickyModifierToggle enables/disables sticky modifier toggle.
func (et *EventTap) SetStickyModifierToggle(enabled bool) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.stickyModifierToggle = enabled
	if enabled {
		// Disarm until the platform handler sees every pre-held modifier
		// released, so the activation chord is not read as a sticky toggle.
		et.stickyModifierDetectionArmed = false
	} else {
		et.stickyModifierDetectionArmed = true
	}
}

// PostModifierEvent posts a modifier key event.
func (et *EventTap) PostModifierEvent(modifier string, isDown bool) {
	modifier = keyvocab.CanonicalModifier(modifier)
	if modifier == "" {
		return
	}

	// On X11, XTest modifier events re-enter the tap and must be suppressed
	// so they do not trigger __modifier_ events. On Wayland,
	// zwp_virtual_keyboard_v1_modifiers generates no evdev or wl_keyboard
	// event, and remembering one would suppress a real press. This must match
	// postLinuxModifierEvent's answer, so both ask the one detector.
	onWayland := platform.DetectLinuxBackend().IsWayland()
	if !onWayland {
		et.rememberSyntheticModifierEvent(modifier, isDown)
	}

	if !postLinuxModifierEvent(modifier, isDown) {
		if !onWayland {
			et.consumeSyntheticModifierEvent(modifier, isDown)
		}
	}
}

// syntheticModifierNames is every modifier the tap names, paired with the bit
// an injection path names it by. It must match the set x11ModifierName
// resolves keysyms onto, or a registration misses its event.
var syntheticModifierNames = []struct {
	bit  action.Modifiers
	name string
}{
	{action.ModShift, evdevModifierShift},
	{action.ModCtrl, evdevModifierCtrl},
	{action.ModAlt, evdevModifierAlt},
	{action.ModCmd, evdevModifierCmd},
}

// RememberSyntheticModifier disowns a modifier key event Neru is about to
// inject through its own path, so the tap reads it as Neru's, not the user's.
//
// PostModifierEvent books its own events. The X11 accessibility backend
// presents an action's modifiers by pressing real keys
// (internal/adapter/platform/modifierstate), which re-enter the grab, and a
// press plus release is a modifier tap. Without this the injection can latch a
// sticky modifier the user never pressed (#1484).
//
// Call it immediately before injecting, since the grab reads on its own
// goroutine and could see the event first. Registrations expire after
// syntheticModifierSuppressionWindow, which covers an injection the display
// server refuses. Wayland never needs it, because its virtual-keyboard
// modifiers generate no evdev or wl_keyboard event.
func (et *EventTap) RememberSyntheticModifier(modifier action.Modifiers, isDown bool) {
	name := syntheticModifierName(modifier)
	if name == "" {
		// Unreachable while every key a plan names carries one modifier, and a
		// silent #1484 regression if that changes. Debug, not warn, because
		// the injection is still correct and this is per-keypress.
		if et.logger != nil && modifier != 0 {
			et.logger.Debug(
				"Injected modifier key names no single modifier; leaving it unannounced",
				zap.Stringer("modifiers", modifier),
			)
		}

		return
	}

	et.rememberSyntheticModifierEvent(name, isDown)
}

// syntheticModifierName is the name the grab will read an injected key event
// under, and "" when modifier does not name exactly one.
//
// One key event consumes one registration, so a set naming two would leave an
// entry waiting out the suppression window, long enough to swallow a real
// modifier press. Announcing nothing is the safer failure.
func syntheticModifierName(modifier action.Modifiers) string {
	var name string

	for _, known := range syntheticModifierNames {
		if !modifier.Has(known.bit) {
			continue
		}

		if name != "" {
			return ""
		}

		name = known.name
	}

	return name
}

// x11Yield is one YieldKeyboard request. The tap closes released once it has
// let go of the keyboard, and the caller closes sent once its keys are out.
type x11Yield struct {
	released chan struct{}
	sent     chan struct{}
}

// YieldKeyboard runs send with the X11 keyboard grab released, and the tap
// takes the grab back once send returns. A key the user presses in that moment
// goes to the focused window. With no grab held, as on Wayland or with no mode
// open, send runs as is.
func (et *EventTap) YieldKeyboard(send func() error) error {
	err := et.KeysReachApp()
	if err != nil {
		return err
	}

	if !et.x11Grabbed.Load() {
		return send()
	}

	et.mu.RLock()
	doneCh := et.doneCh
	et.mu.RUnlock()

	request := x11Yield{released: make(chan struct{}), sent: make(chan struct{})}

	select {
	case et.yieldCh <- request:
	case <-doneCh:
		return send()
	}

	defer close(request.sent)

	<-request.released

	return send()
}

// errTypedKeysComeBack refuses a typed key while the Wayland fallback holds
// the keyboard, where it would come straight back to Neru.
var errTypedKeysComeBack = derrors.New(
	derrors.CodeNotSupported,
	"typing into the app needs a readable /dev/input and a writable /dev/uinput",
)

// KeysReachApp reports whether a key typed now would reach the focused app.
// It refuses while the Wayland fallback holds the keyboard through Neru's own
// overlay, since a typed key would come back to Neru.
func (et *EventTap) KeysReachApp() error {
	if et.overlayCapture.Load() {
		return errTypedKeysComeBack
	}

	return nil
}

// IsEnabled returns whether interception is active.
func (et *EventTap) IsEnabled() bool {
	et.mu.RLock()
	defer et.mu.RUnlock()

	return et.enabled
}

// stickyArmDetection arms sticky modifier detection. The platform handler
// calls it once all pre-held modifiers are released.
func (et *EventTap) stickyArmDetection() {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.stickyModifierDetectionArmed = true
}

func (et *EventTap) stickyDetectionArmed() bool {
	et.mu.RLock()
	defer et.mu.RUnlock()

	return et.stickyModifierDetectionArmed
}

// shouldPassthroughChord reports whether an unbound modifier chord should go
// to the focused app instead of Neru, mirroring macOS. Passthrough must be
// enabled, the chord must carry Ctrl, Alt or Cmd (shift-only chords stay
// usable in modes), and it must be neither blacklisted nor intercepted.
func (et *EventTap) shouldPassthroughChord(chord string) bool {
	// Check the enabled flag before the allocating normalization, so the
	// default disabled case costs nothing on the key path.
	et.mu.RLock()
	enabled := et.passthroughEnabled
	et.mu.RUnlock()

	if !enabled || !config.HasPassthroughModifier(chord) {
		return false
	}

	canonical := tap.CanonicalChord(chord)

	et.mu.RLock()
	defer et.mu.RUnlock()

	if _, ok := et.passthroughBlacklist[canonical]; ok {
		return false
	}

	if _, ok := et.interceptedChords[canonical]; ok {
		return false
	}

	return true
}

// firePassthroughCallback invokes the passthrough callback, if any, on its own
// goroutine without et.mu, so it can neither block the key reader nor
// deadlock against the mode handler's lock, as on macOS.
func (et *EventTap) firePassthroughCallback() {
	et.mu.RLock()
	cb := et.passthroughCallback
	et.mu.RUnlock()

	if cb != nil {
		go cb()
	}
}

// run starts the event interception loop: evdev under a Wayland compositor, a
// keyboard grab on X11. The choice comes from platform.DetectLinuxBackend,
// not the environment (#1429). A backend the factory refuses runs the X11
// loop, which finds no DISPLAY and returns.
func (et *EventTap) run() {
	if platform.DetectLinuxBackend().IsWayland() {
		et.runWayland()
	} else {
		et.runX11()
	}
}

// dispatchKey enqueues a key for the dispatch goroutine, so the event-tap
// goroutine never blocks on the callback (see dispatchCh).
func (et *EventTap) dispatchKey(key string) {
	if key == "" {
		return
	}

	et.mu.RLock()
	destroyed := et.destroyed
	et.mu.RUnlock()

	if destroyed {
		return
	}

	select {
	case et.dispatchCh <- key:
	default:
		if et.logger != nil && et.dropWarned.CompareAndSwap(false, true) {
			et.logger.Warn("Dispatch channel full, dropping keys")
		}
	}
}

// dispatchLoop reads keys from the dispatch channel and invokes the callback,
// for the EventTap's lifetime.
func (et *EventTap) dispatchLoop() {
	defer et.dispatchWg.Done()

	for key := range et.dispatchCh {
		epoch := et.dispatchEpoch.Load()

		et.mu.RLock()
		cb := et.callback
		et.mu.RUnlock()

		if cb != nil && et.dispatchEpoch.Load() == epoch {
			cb(key)
		}

		// A drained channel re-arms the warning for the next overflow.
		if len(et.dispatchCh) == 0 && et.dropWarned.Load() {
			et.dropWarned.Store(false)
		}
	}
}

func (et *EventTap) stickyToggleEnabled() bool {
	et.mu.RLock()
	defer et.mu.RUnlock()

	return et.stickyModifierToggle
}

func (et *EventTap) rememberSyntheticModifierEvent(modifier string, isDown bool) {
	now := time.Now()

	et.mu.Lock()
	defer et.mu.Unlock()

	pending := make([]pendingSyntheticModifierEvent, 0, len(et.syntheticModifierEvents))
	for _, event := range et.syntheticModifierEvents {
		if now.Before(event.expiresAt) {
			pending = append(pending, event)
		}
	}

	pending = append(pending, pendingSyntheticModifierEvent{
		modifier:  modifier,
		isDown:    isDown,
		expiresAt: now.Add(syntheticModifierSuppressionWindow),
	})
	et.syntheticModifierEvents = pending
}

func (et *EventTap) consumeSyntheticModifierEvent(modifier string, isDown bool) bool {
	now := time.Now()

	et.mu.Lock()
	defer et.mu.Unlock()

	pending := make([]pendingSyntheticModifierEvent, 0, len(et.syntheticModifierEvents))
	consumed := false

	for _, event := range et.syntheticModifierEvents {
		if !now.Before(event.expiresAt) {
			continue
		}

		if !consumed && event.modifier == modifier && event.isDown == isDown {
			consumed = true

			continue
		}

		pending = append(pending, event)
	}

	et.syntheticModifierEvents = pending

	return consumed
}
