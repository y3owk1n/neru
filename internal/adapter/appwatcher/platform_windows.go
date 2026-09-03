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
	wg      sync.WaitGroup

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
	l.lastID, l.lastName = "", ""

	unhook, err := l.subscribe(func(hwnd uintptr) { offer(events, hwnd) })
	if err != nil {
		l.watcher.logger.Warn(
			"App watcher: foreground hook install failed; per-app config settles when a mode opens",
			zap.Error(err),
		)

		return
	}

	l.unhook = unhook
	l.stopCh = make(chan struct{})

	l.wg.Add(1)

	go l.loop(l.stopCh, events)

	offer(events, l.foreground())
}

// stop unhooks first, so nothing offers after the loop is told to exit, then
// joins the loop.
func (l *windowsAppWatcher) stop() {
	l.mu.Lock()

	if l.stopCh == nil {
		l.mu.Unlock()

		return
	}

	stopCh, unhook := l.stopCh, l.unhook
	l.stopCh, l.unhook = nil, nil

	l.mu.Unlock()

	unhook()
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

func (l *windowsAppWatcher) loop(stopCh <-chan struct{}, events <-chan uintptr) {
	defer l.wg.Done()

	for {
		select {
		case <-stopCh:
			return
		case hwnd := <-events:
			l.tick(hwnd)
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
