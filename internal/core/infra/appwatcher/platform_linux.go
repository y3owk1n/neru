//go:build linux

// internal/core/infra/appwatcher/platform_linux.go
// Linux app-watcher backend. macOS receives focus changes from an NSWorkspace
// observer; on Linux the focused application's identity is the app_id (Wayland
// wlroots/KDE) or WM_CLASS (X11). Where the compositor/X11 exposes a
// focus-change notification (via linux.SubscribeFocusedApp), the watcher blocks
// on that fd and re-samples on each wake — near-instant per-app hotkey
// re-registration. Otherwise it falls back to polling on a fixed interval.
// GNOME/Mutter exposes no focused-app source, so no events fire there.

package appwatcher

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/y3owk1n/neru/internal/core/infra/platform"
	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
)

// focusPollInterval is how often the focused application is sampled on the
// polling fallback (backends with no focus-change fd). It trades switch latency
// (per-app hotkeys re-register on the next poll after an alt-tab) against idle
// cost (one lightweight focus query per interval).
const focusPollInterval = 400 * time.Millisecond

// focusEventPollTimeout bounds a single poll() wait on the event-driven path so
// the loop notices context cancellation within this window even when no focus
// event arrives. It is short enough for prompt shutdown yet long enough that an
// idle daemon makes only a couple of no-op poll() syscalls per second.
const focusEventPollTimeout = 500 * time.Millisecond

// focusEventSafetyInterval bounds how long the event-driven loop goes without a
// forced re-sample. Focus changes normally wake it immediately via the fd; this
// periodic re-sample is a safety net against a missed or coalesced event and
// costs one focus query per interval (matching the polling fallback's idle
// cost, but far less often).
const focusEventSafetyInterval = 3 * time.Second

// globalLinuxWatcher is the process-wide app watcher, mirroring the single
// darwin.SetAppWatcher registration. NewWatcher registers itself here via
// platformRegisterWatcher.
var globalLinuxWatcher = &linuxAppWatcher{
	identity:        linux.FocusedAppID,
	subscribe:       linux.SubscribeFocusedApp,
	subscribeScreen: linux.SubscribeScreenChange,
	refreshScreens:  linux.RefreshScreens,
	interval:        focusPollInterval,
}

// linuxAppWatcher samples the focused-application identity and dispatches
// activation changes to the registered Watcher, waking on a focus-change fd
// where one is available and polling otherwise.
type linuxAppWatcher struct {
	// identity resolves the focused app_id for a backend; injectable for tests.
	identity func(backend string) (string, bool)
	// subscribe returns an fd that becomes readable on focus changes, or
	// ok=false to select the polling fallback; injectable for tests. A nil
	// subscribe also selects polling.
	subscribe func(backend string) (int, bool)
	// subscribeScreen returns an fd that becomes readable on display-configuration
	// changes (monitor hotplug/resize), or ok=false when none is available.
	// A nil subscribeScreen disables screen-change watching (e.g. in tests).
	subscribeScreen func(backend string) (int, bool)
	// refreshScreens re-reads the display layout into the platform cache after a
	// screen-change event, before the screen-parameters callback runs.
	refreshScreens func(backend string)
	interval       time.Duration

	mu      sync.Mutex
	watcher *Watcher
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// last is the most recently dispatched app_id ("" means "none focused").
	// It is owned exclusively by the sample goroutine, so it needs no lock.
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

	// Watch display-configuration changes on a sibling goroutine so monitor
	// hotplug/resize regenerates overlays without waiting on the focus loop.
	if l.subscribeScreen != nil {
		l.wg.Add(1)

		go l.loopScreen(ctx, backend)
	}
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

// loop samples the focused application until the context is canceled. It
// samples once immediately, then either blocks on a focus-change fd (when the
// backend exposes one) or polls on a fixed interval.
func (l *linuxAppWatcher) loop(ctx context.Context, backend string) {
	defer l.wg.Done()

	// Sample once immediately so per-app state converges without waiting for the
	// first event or poll tick.
	l.tick(backend)

	if l.subscribe != nil {
		if fd, ok := l.subscribe(backend); ok && fd >= 0 {
			l.watcher.logger.Debug("App watcher: using event-driven focus updates",
				zap.String("backend", backend))
			l.loopEvent(ctx, backend, fd)

			return
		}
	}

	l.watcher.logger.Debug("App watcher: polling focus updates",
		zap.String("backend", backend),
		zap.Duration("interval", l.interval))
	l.loopPoll(ctx, backend)
}

// loopPoll samples the focused application every interval until the context is
// canceled. Used when the backend exposes no focus-change fd.
func (l *linuxAppWatcher) loopPoll(ctx context.Context, backend string) {
	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.tick(backend)
		}
	}
}

// loopEvent blocks on the focus-change fd and re-samples on each wake. A short
// poll timeout keeps shutdown responsive to context cancellation, and a
// periodic safety-net re-sample guards against a missed or coalesced event. If
// the fd hangs up or errors, it degrades to polling so focus tracking survives.
//
// focusFD is owned by the platform layer (a process-lifetime pipe/monitor).
// We poll a dup of it so this goroutine holds its own descriptor: if that layer
// ever closes the underlying fd, our in-flight Poll can't become a
// use-after-close or fd-reuse race — we simply see POLLHUP and fall back to
// polling. dup(2) clears close-on-exec, so we re-arm it; neru spawns exec-action
// children that must not inherit this fd.
func (l *linuxAppWatcher) loopEvent(ctx context.Context, backend string, focusFD int) {
	dupFD, err := unix.Dup(focusFD)
	if err != nil {
		l.watcher.logger.Warn("App watcher: dup focus fd failed, falling back to polling",
			zap.String("backend", backend),
			zap.Error(err))
		l.loopPoll(ctx, backend)

		return
	}

	unix.CloseOnExec(dupFD)

	defer func() { _ = unix.Close(dupFD) }()

	pollFDs := []unix.PollFd{{Fd: int32(dupFD), Events: unix.POLLIN}}
	nextSafety := time.Now().Add(focusEventSafetyInterval)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, err = unix.Poll(pollFDs, int(focusEventPollTimeout/time.Millisecond))
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			// Unexpected poll failure — degrade to polling rather than spin.
			l.watcher.logger.Warn("App watcher: focus fd poll failed, falling back to polling",
				zap.String("backend", backend),
				zap.Error(err))
			l.loopPoll(ctx, backend)

			return
		}

		revents := pollFDs[0].Revents

		if revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			l.watcher.logger.Warn("App watcher: focus fd hung up, falling back to polling",
				zap.String("backend", backend))
			l.loopPoll(ctx, backend)

			return
		}

		if revents&unix.POLLIN != 0 {
			drainFD(dupFD)
			l.tick(backend)

			nextSafety = time.Now().Add(focusEventSafetyInterval)

			continue
		}

		// Poll timed out with no event: re-sample only when the safety interval
		// has elapsed, so an idle X11 daemon isn't opening a display every tick.
		if !time.Now().Before(nextSafety) {
			l.tick(backend)

			nextSafety = time.Now().Add(focusEventSafetyInterval)
		}
	}
}

// loopScreen blocks on the display-configuration-change fd and, on each wake,
// refreshes the platform's screen cache and dispatches a screen-parameters
// change to the Watcher (which regenerates overlays for the new layout). This is
// the Linux equivalent of macOS's NSApplicationDidChangeScreenParameters
// notification. When no fd is available (GNOME/Mutter, RandR-less X, or a nil
// subscribe) it exits immediately — screen changes simply go unobserved, and the
// overlay still follows the cursor. On fd hangup/error it exits rather than
// spinning, since there is no cheap polling equivalent for layout changes.
//
// The fd is owned by the platform layer; we poll a dup so a close on that side
// surfaces as POLLHUP instead of a use-after-close, mirroring loopEvent.
func (l *linuxAppWatcher) loopScreen(ctx context.Context, backend string) {
	defer l.wg.Done()

	screenFD, ok := l.subscribeScreen(backend)
	if !ok || screenFD < 0 {
		l.watcher.logger.Debug("App watcher: no screen-change fd; display hotplug events disabled",
			zap.String("backend", backend))

		return
	}

	dupFD, err := unix.Dup(screenFD)
	if err != nil {
		l.watcher.logger.Warn("App watcher: dup screen fd failed; display hotplug events disabled",
			zap.String("backend", backend),
			zap.Error(err))

		return
	}

	unix.CloseOnExec(dupFD)

	defer func() { _ = unix.Close(dupFD) }()

	l.watcher.logger.Debug("App watcher: using event-driven screen-change updates",
		zap.String("backend", backend))

	pollFDs := []unix.PollFd{{Fd: int32(dupFD), Events: unix.POLLIN}}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, err = unix.Poll(pollFDs, int(focusEventPollTimeout/time.Millisecond))
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}

			l.watcher.logger.Warn(
				"App watcher: screen fd poll failed; display hotplug events disabled",
				zap.String("backend", backend),
				zap.Error(err),
			)

			return
		}

		revents := pollFDs[0].Revents

		if revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			l.watcher.logger.Warn("App watcher: screen fd hung up; display hotplug events disabled",
				zap.String("backend", backend))

			return
		}

		if revents&unix.POLLIN != 0 {
			drainFD(dupFD)

			// Refresh the platform screen cache before the callback re-queries
			// ScreenBounds/ScreenNames for the new layout.
			if l.refreshScreens != nil {
				l.refreshScreens(backend)
			}

			l.watcher.logger.Debug("App watcher: display configuration changed",
				zap.String("backend", backend))
			l.watcher.HandleScreenParametersChanged()
		}
	}
}

// drainFD reads and discards all pending bytes from a non-blocking fd so a
// burst of focus-change signals coalesces into a single re-sample.
func drainFD(fd int) {
	var buf [64]byte

	for {
		n, err := unix.Read(fd, buf[:])
		if n <= 0 || err != nil {
			return
		}

		if n < len(buf) {
			return
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
