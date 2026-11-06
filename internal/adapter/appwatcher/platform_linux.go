//go:build linux

package appwatcher

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
)

// Linux app-watcher backend. The focused application's identity is the
// app_id (Wayland) or WM_CLASS (X11). Where the backend exposes a focus-change
// fd (linux.SubscribeFocusedApp), the watcher blocks on it and re-samples on
// each wake. Otherwise it polls.
//
// focusPollInterval is the polling fallback's period, trading alt-tab latency
// for per-app hotkeys against one focus query per interval.
const focusPollInterval = 400 * time.Millisecond

// focusEventPollTimeout bounds a single poll() wait on the event-driven path so
// the loop notices context cancellation promptly.
const focusEventPollTimeout = 500 * time.Millisecond

// focusEventSafetyInterval is how often the event-driven loop re-samples
// without an event, a safety net against a missed or coalesced one.
const focusEventSafetyInterval = 3 * time.Second

// globalLinuxWatcher is the process-wide app watcher, mirroring the single
// darwin.SetAppWatcher registration.
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
	// identity resolves the focused app_id for a backend. Tests inject it.
	identity func(backend string) (string, bool)
	// subscribe returns an fd that becomes readable on focus changes, or
	// ok=false (or nil) to select the polling fallback.
	subscribe func(backend string) (int, bool)
	// subscribeScreen returns an fd that becomes readable on display-configuration
	// changes, or ok=false when none is available. Nil disables screen watching.
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

// start launches the poll loop. It is idempotent.
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

// loop samples the focused application until the context is canceled, once
// immediately, then on a focus-change fd or a fixed interval.
func (l *linuxAppWatcher) loop(ctx context.Context, backend string) {
	defer l.wg.Done()

	// Sample at once so per-app state converges before the first event.
	l.tick(backend)

	if l.subscribe != nil {
		if fd, ok := l.subscribe(backend); ok && fd >= 0 {
			l.watcher.logger.Debug("Using event-driven focus updates",
				zap.String("backend", backend))
			l.loopEvent(ctx, backend, fd)

			return
		}
	}

	l.watcher.logger.Debug("Polling focus updates",
		zap.String("backend", backend),
		zap.Duration("interval", l.interval))
	l.loopPoll(ctx, backend)
}

// loopPoll samples the focused application every interval until the context is
// canceled.
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

// loopEvent blocks on the focus-change fd and re-samples on each wake, with a
// periodic safety-net re-sample. If the fd hangs up or errors, it degrades to
// polling.
//
// The platform layer owns focusFD. We poll a dup, so if that layer closes it
// we see POLLHUP instead of a use-after-close or fd-reuse race. dup(2) clears
// close-on-exec, so it is re-armed, because exec-action children must not
// inherit this fd.
func (l *linuxAppWatcher) loopEvent(ctx context.Context, backend string, focusFD int) {
	dupFD, err := unix.Dup(focusFD)
	if err != nil {
		l.watcher.logger.Warn("Dup focus fd failed, falling back to polling",
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
			// Unexpected poll failure. Degrade to polling rather than spin.
			l.watcher.logger.Warn("Focus fd poll failed, falling back to polling",
				zap.String("backend", backend),
				zap.Error(err))
			l.loopPoll(ctx, backend)

			return
		}

		revents := pollFDs[0].Revents

		if revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			l.watcher.logger.Warn("Focus fd hung up, falling back to polling",
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

		// Poll timed out with no event. Re-sample only when the safety interval
		// has elapsed, so an idle X11 daemon isn't opening a display every tick.
		if !time.Now().Before(nextSafety) {
			l.tick(backend)

			nextSafety = time.Now().Add(focusEventSafetyInterval)
		}
	}
}

// loopScreen blocks on the display-configuration-change fd and, on each wake,
// refreshes the platform's screen cache and dispatches a screen-parameters
// change (the Linux NSApplicationDidChangeScreenParameters). With no fd
// (GNOME/Mutter, RandR-less X, or a nil subscribe) it exits at once and the
// overlay still follows the cursor. On hangup/error it exits rather than spin,
// since layout changes have no cheap polling equivalent. It polls a dup of the
// fd, as loopEvent does.
func (l *linuxAppWatcher) loopScreen(ctx context.Context, backend string) {
	defer l.wg.Done()

	screenFD, ok := l.subscribeScreen(backend)
	if !ok || screenFD < 0 {
		l.watcher.logger.Debug("No screen-change fd; display hotplug events disabled",
			zap.String("backend", backend))

		return
	}

	dupFD, err := unix.Dup(screenFD)
	if err != nil {
		l.watcher.logger.Warn("Dup screen fd failed; display hotplug events disabled",
			zap.String("backend", backend),
			zap.Error(err))

		return
	}

	unix.CloseOnExec(dupFD)

	defer func() { _ = unix.Close(dupFD) }()

	l.watcher.logger.Debug("Using event-driven screen-change updates",
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
				"Screen fd poll failed; display hotplug events disabled",
				zap.String("backend", backend),
				zap.Error(err),
			)

			return
		}

		revents := pollFDs[0].Revents

		if revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			l.watcher.logger.Warn("Screen fd hung up; display hotplug events disabled",
				zap.String("backend", backend))

			return
		}

		if revents&unix.POLLIN != 0 {
			drainFD(dupFD)

			// Refresh the cache before the callback re-queries the layout.
			if l.refreshScreens != nil {
				l.refreshScreens(backend)
			}

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

	// On Linux the app_id is the only identity available, so it is both
	// the human-facing app name and the per-app bundle identifier.
	if prev != "" {
		l.watcher.HandleDeactivate(prev, prev)
	}

	if appID != "" {
		l.watcher.HandleActivate(appID, appID)
	}
}

func platformRegisterWatcher(w *Watcher) { globalLinuxWatcher.register(w) }
func platformStartWatcher()              { globalLinuxWatcher.start() }
func platformStopWatcher()               { globalLinuxWatcher.stop() }

// platformSetMCDetection is a no-op on Linux, because Mission Control is a macOS
// concept with no Linux equivalent.
func platformSetMCDetection(_ bool) {}
