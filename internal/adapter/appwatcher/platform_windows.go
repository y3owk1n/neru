//go:build windows

package appwatcher

import (
	"sync"

	"go.uber.org/zap"

	winplatform "github.com/y3owk1n/neru/internal/adapter/platform/windows"
)

// Windows app-watcher backend. The platform layer installs an
// EVENT_SYSTEM_FOREGROUND hook (winplatform.StartForegroundWatcher) whose
// callback runs on the hook's own message-loop thread. That callback only
// offers the new HWND to a one-slot channel, latest wins, and a goroutine
// owned by this backend resolves it to an application identity and dispatches
// the change — so callbacks registered on the Watcher never run on the pump
// thread and never run inside a Win32 callback (internal/app/modes/AGENTS.md,
// ADR 0005). The identity is the executable path, the same value
// GetForegroundWindow resolves to for the focused app, so per-app
// configuration keys on one string whichever way it is learned.
//
// Display changes take the same route from a second source: a hidden window
// (winplatform.StartDisplayWatcher) receives WM_DISPLAYCHANGE and
// WM_DPICHANGED on its own pump thread, offers a token to a one-slot channel,
// and the dispatch goroutine turns it into HandleScreenParametersChanged —
// the event macOS's screen-parameters notification and Linux's RandR fd
// produce, so the app re-lays-out the overlay the same way on all three.
//
// globalWindowsWatcher is the process-wide backend, mirroring
// globalLinuxWatcher. NewWatcher registers itself here via
// platformRegisterWatcher.
var globalWindowsWatcher = &windowsAppWatcher{
	subscribe: func(callback func(uintptr)) (func(), error) {
		hook, err := winplatform.StartForegroundWatcher(callback)
		if err != nil {
			return nil, err
		}

		return hook.Stop, nil
	},
	subscribeDisplay: func(callback func()) (func(), error) {
		watcher, err := winplatform.StartDisplayWatcher(callback)
		if err != nil {
			return nil, err
		}

		return watcher.Stop, nil
	},
	foreground: func() uintptr {
		hwnd, _ := winplatform.ForegroundWindowHandle()

		return hwnd
	},
	identity: winplatform.WindowApplicationIdentity,
}

// windowsAppWatcher turns foreground-window changes into activate and
// deactivate dispatches on the registered Watcher.
type windowsAppWatcher struct {
	// subscribe installs the foreground hook and returns its stop function;
	// injectable for tests.
	subscribe func(callback func(uintptr)) (func(), error)
	// subscribeDisplay installs the display-change window and returns its stop
	// function; injectable for tests.
	subscribeDisplay func(callback func()) (func(), error)
	// foreground samples the current foreground HWND once at start, so the
	// keymap has a published app before the first switch.
	foreground func() uintptr
	// identity resolves an HWND to (appName, bundleID); ok=false means no
	// application has focus.
	identity func(hwnd uintptr) (string, string, bool)

	mu      sync.Mutex
	watcher *Watcher
	stopCh  chan struct{}
	unhook  func()
	// unhookDisplay stops the display watcher; nil when it failed to install.
	unhookDisplay func()
	wg            sync.WaitGroup

	// lastID and lastName are the most recently dispatched identity ("" means
	// none focused). Owned by the dispatch goroutine, so they need no lock.
	lastID   string
	lastName string
}

func (l *windowsAppWatcher) register(w *Watcher) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.watcher = w
}

// start installs the hook and launches the dispatch goroutine. Idempotent.
// A hook that fails to install leaves the watcher idle and says so: the
// keymap then asks the platform when it settles, as it did before the
// watcher existed.
func (l *windowsAppWatcher) start() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.watcher == nil || l.stopCh != nil {
		return
	}

	events := make(chan uintptr, 1)
	displayEvents := make(chan struct{}, 1)
	l.lastID, l.lastName = "", ""

	unhook, err := l.subscribe(func(hwnd uintptr) { offer(events, hwnd) })
	if err != nil {
		l.watcher.logger.Warn(
			"App watcher: foreground hook install failed; per-app config settles when a mode opens",
			zap.Error(err),
		)

		return
	}

	// The app in front now is published without waiting for a switch. The
	// hook is live first so no change slips between sample and install, the
	// snapshot yields to a hook event already in the slot because that event
	// is newer, and the loop starts only afterwards so nothing is consumed
	// between the two — any interleaving publishes the newest identity last.
	select {
	case events <- l.foreground():
	default:
	}

	// A display watcher that fails to install costs only hotplug: the
	// foreground hook is already live and the overlay is still re-sized on
	// each activation, so the watcher runs on without it and says so.
	unhookDisplay, err := l.subscribeDisplay(func() { offerToken(displayEvents) })
	if err != nil {
		l.watcher.logger.Warn(
			"App watcher: display-change window install failed; overlays follow display changes on the next activation only",
			zap.Error(err),
		)
	}

	l.unhook = unhook
	l.unhookDisplay = unhookDisplay
	l.stopCh = make(chan struct{})

	l.wg.Add(1)

	go l.loop(l.stopCh, events, displayEvents)
}

// stop unhooks first, so nothing offers after the loop is told to exit, then
// joins the loop.
func (l *windowsAppWatcher) stop() {
	l.mu.Lock()

	if l.stopCh == nil {
		l.mu.Unlock()

		return
	}

	stopCh, unhook, unhookDisplay := l.stopCh, l.unhook, l.unhookDisplay
	l.stopCh, l.unhook, l.unhookDisplay = nil, nil, nil

	l.mu.Unlock()

	unhook()

	if unhookDisplay != nil {
		unhookDisplay()
	}

	close(stopCh)
	l.wg.Wait()
}

// offer hands an HWND to the dispatch goroutine without blocking: a burst of
// foreground changes coalesces to the newest one, which is the only one whose
// identity still matters. It runs on the hook's pump thread, which is why it
// takes the channel rather than reading it off the backend.
func offer(events chan uintptr, hwnd uintptr) {
	for {
		select {
		case events <- hwnd:
			return
		default:
		}

		select {
		case <-events:
		default:
		}
	}
}

// offerToken is offer for the display channel: a burst of display messages
// coalesces to a single pending refresh, because the refresh re-reads the
// display and has nothing to learn from the count. It runs on the display
// watcher's pump thread.
func offerToken(events chan struct{}) {
	select {
	case events <- struct{}{}:
	default:
	}
}

func (l *windowsAppWatcher) loop(
	stopCh <-chan struct{},
	events <-chan uintptr,
	displayEvents <-chan struct{},
) {
	defer l.wg.Done()

	for {
		select {
		case <-stopCh:
			return
		case hwnd := <-events:
			l.tick(hwnd)
		case <-displayEvents:
			l.watcher.HandleScreenParametersChanged()
		}
	}
}

// tick resolves one foreground HWND and dispatches the change it represents.
// A transition to "no focused app" emits a deactivate with no activate.
func (l *windowsAppWatcher) tick(hwnd uintptr) {
	name, appID, ok := l.identity(hwnd)
	if !ok {
		name, appID = "", ""
	}

	if appID == l.lastID {
		return
	}

	prevID, prevName := l.lastID, l.lastName
	l.lastID, l.lastName = appID, name

	if prevID != "" {
		l.watcher.HandleDeactivate(prevName, prevID)
	}

	if appID != "" {
		l.watcher.HandleActivate(name, appID)
	}
}

func platformRegisterWatcher(w *Watcher) { globalWindowsWatcher.register(w) }
func platformStartWatcher()              { globalWindowsWatcher.start() }
func platformStopWatcher()               { globalWindowsWatcher.stop() }

// platformSetMCDetection is a no-op on Windows: Mission Control is a macOS
// concept with no Windows equivalent.
func platformSetMCDetection(_ bool) {}
