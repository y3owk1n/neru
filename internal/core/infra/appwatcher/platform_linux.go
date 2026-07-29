//go:build linux

// internal/core/infra/appwatcher/platform_linux.go
// Linux app-watcher backend. macOS receives focus changes from an NSWorkspace
// observer; Linux has no equivalent push API, so this polls the focused
// application's identity (app_id on Wayland wlroots/KDE, WM_CLASS on X11) and
// dispatches activate/deactivate events when it changes. GNOME/Mutter exposes
// no focused-app source, so the poll yields nothing and no events fire there.

package appwatcher

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/platform"
	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
)

// focusPollInterval is how often the focused application is sampled. It trades
// switch latency (per-app hotkeys re-register on the next poll after an
// alt-tab) against idle cost (one lightweight focus query per interval).
const focusPollInterval = 400 * time.Millisecond

// globalLinuxWatcher is the process-wide app watcher, mirroring the single
// darwin.SetAppWatcher registration. NewWatcher registers itself here via
// platformRegisterWatcher.
var globalLinuxWatcher = &linuxAppWatcher{
	identity: linux.FocusedAppID,
	interval: focusPollInterval,
}

// linuxAppWatcher polls the focused-application identity and dispatches
// activation changes to the registered Watcher.
type linuxAppWatcher struct {
	// identity resolves the focused app_id for a backend; injectable for tests.
	identity func(backend string) (string, bool)
	interval time.Duration

	mu      sync.Mutex
	watcher *Watcher
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// last is the most recently dispatched app_id ("" means "none focused").
	// It is owned exclusively by the poll goroutine, so it needs no lock.
	last string
}

// register records the Watcher that poll events are dispatched to.
func (l *linuxAppWatcher) register(w *Watcher) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.watcher = w
}

// start launches the poll loop. It is idempotent: a second call while already
// running is a no-op.
func (l *linuxAppWatcher) start() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.watcher == nil || l.cancel != nil {
		return
	}

	backend := platform.DetectLinuxBackend().String()

	ctx, cancel := context.WithCancel(context.Background())
	l.cancel = cancel
	l.last = ""

	l.wg.Add(1)

	go l.loop(ctx, backend)
}

// stop halts the poll loop and waits for it to exit.
func (l *linuxAppWatcher) stop() {
	l.mu.Lock()

	if l.cancel == nil {
		l.mu.Unlock()

		return
	}

	cancel := l.cancel
	l.cancel = nil

	l.mu.Unlock()

	cancel()
	l.wg.Wait()
}

// loop samples the focused application every interval until the context is
// canceled, dispatching a deactivate/activate pair on each change.
func (l *linuxAppWatcher) loop(ctx context.Context, backend string) {
	defer l.wg.Done()

	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()

	// Sample once immediately so per-app state converges without waiting a
	// full interval for the first tick.
	l.tick(backend)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.tick(backend)
		}
	}
}

// tick samples the focused application once and dispatches activation changes.
// A transition to "no focused app" (ok == false) emits a deactivate for the
// previous app with no matching activate.
func (l *linuxAppWatcher) tick(backend string) {
	appID, ok := l.identity(backend)
	if !ok {
		appID = ""
	}

	if appID == l.last {
		return
	}

	prev := l.last
	l.last = appID

	// On Linux the app_id is the only identity available, so it serves as both
	// the human-facing app name and the per-app bundle identifier.
	if prev != "" {
		l.watcher.HandleDeactivate(prev, prev)
	}

	if appID != "" {
		l.watcher.logger.Debug("App watcher: focused app changed",
			zap.String("app_id", appID),
			zap.String("previous", prev),
			zap.String("backend", backend))
		l.watcher.HandleActivate(appID, appID)
	}
}

func platformRegisterWatcher(w *Watcher) { globalLinuxWatcher.register(w) }
func platformStartWatcher()              { globalLinuxWatcher.start() }
func platformStopWatcher()               { globalLinuxWatcher.stop() }

// platformSetMCDetection is a no-op on Linux: Mission Control is a macOS
// concept with no Linux equivalent.
func platformSetMCDetection(_ bool) {}
