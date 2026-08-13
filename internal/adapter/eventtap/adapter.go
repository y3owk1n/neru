package eventtap

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
	"github.com/y3owk1n/neru/internal/ports"
)

// Adapter implements ports.EventTapPort by wrapping the existing EventTap.
//
// Every method that drives the tap holds mu, and mu sits below the mode
// handler's own lock: a focus change pushes the passthrough lists down that
// edge (`internal/app/modes/AGENTS.md`). So nothing holding mu may wait on
// anything that takes the handler's lock — which is the whole reason Destroy
// tears the tap down outside it, and the reason destroyed exists.
type Adapter struct {
	tap    tap.Tap
	logger *zap.Logger
	mu     sync.RWMutex
	// destroyed is set by Destroy before it lets go of mu. It is what keeps a
	// caller racing a shutdown out of a tap that is being torn down, now that
	// the lock no longer serializes them: every method that takes mu to drive
	// the tap returns early on it. Enable and Disable say so at debug as well,
	// because they answer their caller with a nil error and would otherwise
	// report a success that did not happen; the setters answer nothing, so
	// there is nothing to correct.
	destroyed bool
	// teardownDone is closed once the tap teardown has returned. It is what a
	// second Destroy waits on, so the method keeps its postcondition — the tap
	// is down when it returns — for a caller that raced the first one, which
	// the lock used to give for free.
	teardownDone chan struct{}
	enabled      bool
}

// NewAdapter creates a new event tap adapter.
func NewAdapter(tap tap.Tap, logger *zap.Logger) *Adapter {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Adapter{
		tap:    tap,
		logger: logger.Named("eventtap.adapter"),
	}
}

// Enable enables the event tap.
//
// After Destroy it is a no-op rather than an error: a mode exiting into a
// teardown is a race the shutdown already won, not a failure the user needs
// told about, and its callers log an error for anything that comes back.
func (a *Adapter) Enable(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.destroyed {
		a.logger.Debug("Enable ignored: the event tap has been destroyed")

		return nil
	}

	a.tap.Enable()
	a.enabled = true

	return nil
}

// Disable disables the event tap.
//
// After Destroy it is a no-op, for the reason Enable gives.
func (a *Adapter) Disable(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.destroyed {
		a.logger.Debug("Disable ignored: the event tap has been destroyed")

		return nil
	}

	a.tap.Disable()
	a.enabled = false

	return nil
}

// IsEnabled returns true if event capture is active.
func (a *Adapter) IsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.enabled
}

// SetHandler sets the function to call when a key is pressed.
func (a *Adapter) SetHandler(_ func(key string)) {
	a.logger.Warn("SetHandler called but EventTap doesn't support changing handler after creation")
}

// SetHotkeys configures which hotkeys the event tap should monitor.
// An empty slice is valid and clears all monitored hotkeys.
func (a *Adapter) SetHotkeys(hotkeys []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.destroyed {
		return
	}

	if len(hotkeys) == 0 {
		a.logger.Debug("SetHotkeys called with empty slice — no hotkeys will be monitored")
	}

	a.tap.SetHotkeys(hotkeys)
}

// SetModifierPassthrough configures whether unbound modifier shortcuts should
// pass through to macOS and which shortcuts remain blacklisted.
//
// This is the push a focus change makes, and the one that races a shutdown:
// see the Destroy comment.
func (a *Adapter) SetModifierPassthrough(enabled bool, blacklist []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.destroyed {
		return
	}

	a.tap.SetModifierPassthrough(enabled, blacklist)
}

// SetInterceptedModifierKeys configures modifier shortcuts the active mode
// still wants Neru to consume.
func (a *Adapter) SetInterceptedModifierKeys(keys []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.destroyed {
		return
	}

	a.tap.SetInterceptedModifierKeys(keys)
}

// SetPassthroughCallback registers a function to call when a modifier shortcut
// passes through to macOS.
func (a *Adapter) SetPassthroughCallback(callback func()) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.destroyed {
		return
	}

	a.tap.SetPassthroughCallback(callback)
}

// SetStickyModifierToggle enables or disables sticky modifier toggle detection.
func (a *Adapter) SetStickyModifierToggle(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.destroyed {
		return
	}

	a.tap.SetStickyModifierToggle(enabled)
}

// SetKeyboardLayout configures the reference keyboard layout used by key translation.
//
// Like PostModifierEvent it takes no lock and carries no destroyed guard,
// because no backend routes it through the tap handle: macOS resolves a
// process-wide input source, and Linux and Windows answer true without
// touching anything. So it is neither a caller of the tap being torn down nor
// something a shutdown has to keep out.
func (a *Adapter) SetKeyboardLayout(layoutID string) bool {
	return a.tap.SetKeyboardLayout(layoutID)
}

// PostModifierEvent simulates a physical modifier key press or release.
// No lock is needed because the underlying C function is a standalone utility
// that posts directly to the system without accessing the event tap handle.
func (a *Adapter) PostModifierEvent(modifier string, isDown bool) {
	a.tap.PostModifierEvent(modifier, isDown)
}

// Destroy cleans up the event tap resources. It is safe to call twice, and
// safe from the startup unwind as well as the ordinary shutdown.
//
// The tap teardown runs **outside** mu, and that is the point of the method's
// shape. The macOS and Linux taps spend it waiting for the key dispatcher to
// drain — stopDispatcher there, dispatchWg.Wait() here — and the dispatcher
// they wait for delivers keys into modes.Handler.HandleKeyPress, which takes
// the handler's lock and pushes the passthrough lists straight back out
// through SetModifierPassthrough, which takes mu. Holding mu across the wait
// inverts the documented handler → adapter order, so a shutdown racing a focus
// change deadlocked with neither side able to give way. The Linux tap's own
// Destroy releases its lock before waiting for the same reason. Windows waits
// too, but bounded: its hook join gives up after 250ms and reaps in the
// background, for this same hazard one layer down.
//
// What mu still covers is the state: the adapter marks itself destroyed and
// disabled under it, in one hold, before letting go — so a caller racing the
// teardown finds an adapter that has already stopped answering for the tap
// instead of one whose tap is being freed underneath it. A second caller waits
// on the first one's teardown rather than returning early, because the method
// promises a tap that is down when it returns and the app closes the rest of
// its infrastructure on the strength of that.
//
// Nothing here unlocks mid-method: the state change is claimTeardown's whole
// body, released by its own defer, and this method runs what it was handed
// after that returns. That is the "settle it under the lock, act after the
// release" idiom the handler's guide states, and the reason it is two methods
// rather than one with an explicit Unlock in the middle.
func (a *Adapter) Destroy() {
	teardownDone, ours := a.claimTeardown()
	if !ours {
		<-teardownDone

		return
	}

	defer close(teardownDone)

	a.tap.Destroy()
}

// AllowsOverlayKeyboardPassthrough reports whether an indicator overlay can
// safely drop exclusive keyboard capture.
//
// Both conditions must hold: a uinput scroll device is available to carry the
// scroll events, and no evdev keyboard grab is active — on wlroots an overlay
// grab deactivates the focused toplevel, which breaks the next hints refresh.
// Non-Linux backends and the no-cgo Linux build report false.
func (a *Adapter) AllowsOverlayKeyboardPassthrough() bool {
	return overlayKeyboardPassthroughAllowed()
}

// claimTeardown settles who is tearing the tap down and marks the adapter
// destroyed and disabled in the same hold.
//
// It answers the channel that closes when the teardown is finished, and
// whether this caller is the one that has to run it: false means someone else
// already claimed it and the channel is theirs to wait on.
func (a *Adapter) claimTeardown() (chan struct{}, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.destroyed {
		return a.teardownDone, false
	}

	a.destroyed = true
	a.enabled = false
	a.teardownDone = make(chan struct{})

	return a.teardownDone, true
}

// Ensure Adapter implements ports.EventTapPort and the optional overlay
// passthrough extension.
var (
	_ ports.EventTapPort                       = (*Adapter)(nil)
	_ ports.OverlayKeyboardPassthroughReporter = (*Adapter)(nil)
)
