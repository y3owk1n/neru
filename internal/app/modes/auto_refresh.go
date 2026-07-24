package modes

import (
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
	"github.com/y3owk1n/neru/internal/core/infra/axobserver"
)

const (
	// defaultAutoRefreshDebounce is the debounce window used when the configured
	// hints.auto_refresh.min_refresh_delay_ms is unset or non-positive.
	defaultAutoRefreshDebounce = 150 * time.Millisecond

	// autoRefreshMaxWaitFactor caps how long a burst of changes can keep
	// postponing the re-scan. Every change restarts the debounce timer, so an
	// app whose UI never goes quiet would never re-scan on the debounce alone.
	// Once a burst has lasted this many debounce windows, the re-scan fires
	// anyway, so a continuously-animating app still refreshes at a steady rate.
	autoRefreshMaxWaitFactor = 4
)

// hintAutoRefresh holds the auto-refresh scheduling state: the debounce, the
// settle recheck, and the one timer they share.
//
// Its mutex is a leaf lock. Code holding it never acquires the mode lock, and
// the observer callback takes only this mutex. That matters because hints
// teardown joins the observer thread while holding the mode lock, and a
// callback that needed the mode lock would deadlock it. Methods with the
// Locked suffix require the caller to hold mu, and the others take it
// themselves.
type hintAutoRefresh struct {
	mu sync.Mutex

	// fire runs on timer expiry, carrying the arming generation. The Handler
	// installs its dispatch here at construction. Tests may leave it nil when
	// they drive the state machine directly.
	fire func(gen uint64)

	// timer is shared by the debounce and the settle recheck. timerGen
	// identifies the current arming. Every arm and every teardown bumps it, so
	// a fire whose timer was replaced while the fire waited for the mode lock
	// sees a mismatched generation and touches nothing.
	timer    *time.Timer
	timerGen uint64

	burstOpen   bool
	scanPending bool
	burstStart  time.Time
	debounce    time.Duration
	maxWait     time.Duration
	onFire      func() // test seam; nil in production

	settling        bool
	interval        time.Duration
	floor           time.Duration
	changedCount    int
	stableAtCap     int
	lastFingerprint uint64
}

// setTiming copies the configured debounce window, and the max-wait cap
// derived from it, into this type. The observer callback needs both, and it
// can only read them under this mutex, because the live config sits behind the
// mode lock the callback must never take. Called on every hints activation
// while auto_refresh is enabled.
func (ar *hintAutoRefresh) setTiming(debounce time.Duration) {
	if debounce <= 0 {
		debounce = defaultAutoRefreshDebounce
	}

	ar.mu.Lock()
	ar.debounce = debounce
	ar.maxWait = debounce * autoRefreshMaxWaitFactor
	ar.mu.Unlock()
}

// holdLocked pauses a pending refresh while the user is typing. It cancels the
// timer so nothing scans mid-keystroke. When a settle session is paused, it
// keeps its state and later resumes where it left off. Otherwise the debounce
// is marked as owing a scan. resumeHeld releases the hold when a search ends
// with a confirm or a cancel. When typing ends any other way, the next
// observer event or manual refresh picks up the held work. Caller must hold
// mu.
func (ar *hintAutoRefresh) holdLocked() {
	if ar.timer != nil {
		ar.timer.Stop()
		ar.timer = nil
	}

	if ar.settling {
		return
	}

	ar.burstOpen = true
	ar.scanPending = true
	ar.burstStart = time.Now()
}

// onChange records a UI change reported by the AX observer. It runs on the
// observer callback thread, and hints teardown joins that thread while holding
// the mode lock, so taking the mode lock here would deadlock. It therefore
// only updates this type's state and arms the timer, and the scan runs later,
// when the timer fires. Waiting for the timer also collapses the flurry of
// notifications a single page load emits into one scan.
func (ar *hintAutoRefresh) onChange() {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	now := time.Now()

	// A change arriving during the settle recheck means notifications are
	// flowing again, so the settle ends and the debounce takes over. The
	// debounce guarantees a scan even when changes never pause, because its max
	// wait is measured from the start of the burst. The burst start is set to
	// now, so that clock starts at this change. The scan that follows starts a
	// fresh settle session.
	if ar.settling {
		ar.stopSettleSessionLocked()

		ar.burstOpen = true
		ar.burstStart = now
		ar.scanPending = true
		ar.armBurstTimerLocked(now)

		return
	}

	if !ar.burstOpen {
		ar.burstOpen = true
		ar.burstStart = now
	}

	ar.scanPending = true
	ar.armBurstTimerLocked(now)
}

// armBurstTimerLocked (re)arms the burst timer, firing after the debounce
// window but never later than the max-wait cap measured from the first change
// in the burst. Caller must hold mu.
func (ar *hintAutoRefresh) armBurstTimerLocked(now time.Time) {
	delay := ar.debounce
	if delay <= 0 {
		delay = defaultAutoRefreshDebounce
	}

	if remaining := ar.burstStart.Add(ar.maxWait).Sub(now); remaining < delay {
		delay = remaining
	}

	if delay < 0 {
		delay = 0
	}

	ar.armTimerLocked(delay)
}

// armTimerLocked replaces the shared timer with one firing after delay,
// stamping it with a fresh generation so any previously pending fire proves
// stale. Caller must hold mu.
func (ar *hintAutoRefresh) armTimerLocked(delay time.Duration) {
	if delay < 0 {
		delay = 0
	}

	if ar.timer != nil {
		ar.timer.Stop()
	}

	ar.timerGen++
	gen := ar.timerGen
	ar.timer = time.AfterFunc(delay, func() {
		if ar.fire != nil {
			ar.fire(gen)
		}
	})
}

// resumeHeld re-arms a refresh that was paused while the user was typing. When
// a settle session was paused, it resumes at its current interval. When the
// debounce owes a scan, the debounce timer is re-armed. When nothing is owed,
// or a timer is already running, nothing changes.
func (ar *hintAutoRefresh) resumeHeld() {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	if ar.timer != nil {
		return
	}

	if ar.settling {
		ar.armTimerLocked(ar.interval)

		return
	}

	if ar.scanPending {
		ar.armBurstTimerLocked(time.Now())
	}
}

// stopAll cancels any pending timer and clears the burst and settle state.
// Bumping the generation invalidates a fire already past its timer, so a stale
// fire that lost the race to Stop cannot act on the cleared state. It takes the
// leaf mutex and is safe to call under the mode lock.
func (ar *hintAutoRefresh) stopAll() {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	ar.timerGen++
	ar.burstOpen = false
	ar.scanPending = false
	ar.settling = false
	ar.interval = 0
	ar.floor = 0
	ar.changedCount = 0
	ar.stableAtCap = 0
	ar.lastFingerprint = 0

	if ar.timer != nil {
		ar.timer.Stop()
		ar.timer = nil
	}
}

// initAutoRefresh installs the observer change callback, wires the timer
// dispatch, and seeds the debounce timing, so a hints activation can point the
// observer at the focused app. Nothing is armed until a hints session runs
// with hints.auto_refresh enabled.
func (h *Handler) initAutoRefresh() {
	h.autoRefresh.fire = h.fireHintRefresh
	h.autoRefresh.debounce = defaultAutoRefreshDebounce
	h.autoRefresh.maxWait = defaultAutoRefreshDebounce * autoRefreshMaxWaitFactor
	axobserver.Init(func() { h.autoRefresh.onChange() }, h.logger)
}

func (h *Handler) autoRefreshEnabledLocked() bool {
	return axobserver.Supported() && h.config != nil && h.config.Hints.AutoRefresh.Enabled
}

// admitHintRefresh decides whether a manual in-hints refresh (a --repeat
// re-entry, a passthrough re-scan, a bound `hints` re-launch, a cycle-hint
// re-scan) scans now or waits. It runs while auto_refresh is enabled. When the
// debounce is idle, or its timer was paused for typing, it returns true and
// the caller scans immediately, so a manual refresh is never delayed. When a
// scan already ran inside the current debounce window, it returns false and
// schedules a single trailing scan for when the window closes.
//
// When the user is typing a search query, the refresh is held instead and
// released when the search ends, so the query and its filtered hint set
// survive the keystrokes. A refresh that runs right after the user picks a
// hint label is not held. That one must proceed, because no later event would
// release it, and the overlay would stay frozen on the stale filter. Runs
// under the mode lock and takes only this type's own mutex.
func (h *Handler) admitHintRefresh() bool {
	autoRefresh := &h.autoRefresh

	if h.hintSearchInProgress() {
		autoRefresh.mu.Lock()
		autoRefresh.holdLocked()
		autoRefresh.mu.Unlock()

		return false
	}

	autoRefresh.mu.Lock()
	defer autoRefresh.mu.Unlock()

	now := time.Now()

	// A manual or --repeat refresh outranks a running settle session. The
	// settle stops here so the check below sees no active timer and lets this
	// refresh scan immediately. If the refresh instead waited behind the
	// settle's next check, the overlay would sit frozen on the hint the user
	// just selected. When a later change is observed, a fresh settle starts
	// from the new scan.
	if autoRefresh.settling {
		autoRefresh.stopSettleSessionLocked()
	}

	// Scan immediately when the debounce is idle, and also when its timer was
	// paused for typing (the burst is marked open but no timer runs). Either
	// way the debounce window opens fresh from now. This scan also covers any
	// refresh the observer deferred while the user was typing.
	if !autoRefresh.burstOpen || autoRefresh.timer == nil {
		autoRefresh.burstOpen = true
		autoRefresh.burstStart = now
		autoRefresh.scanPending = false
		autoRefresh.armBurstTimerLocked(now)

		return true
	}

	autoRefresh.scanPending = true
	autoRefresh.armBurstTimerLocked(now)

	return false
}

// fireHintRefresh runs when the shared timer expires, for both the debounce
// and the settle recheck. gen identifies which arming of the timer fired. When
// the timer was re-armed or torn down while this fire waited for the mode
// lock, the generations no longer match, and the fire returns without touching
// anything, so it cannot discard the live timer or run a duplicate scan. When
// the user is mid-typing, the fire holds the refresh instead of scanning, and
// the change waits for typing to end. When a settle session is running, the
// fire runs one settle check. Otherwise it performs the deferred debounce scan
// and starts a settle session. It takes the mode lock first and this type's
// mutex second, never the other way around.
func (h *Handler) fireHintRefresh(gen uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	autoRefresh := &h.autoRefresh

	autoRefresh.mu.Lock()

	if gen != autoRefresh.timerGen {
		autoRefresh.mu.Unlock()

		return
	}

	if h.isMidTyping() {
		autoRefresh.holdLocked()
		autoRefresh.mu.Unlock()

		return
	}

	settling := autoRefresh.settling
	shouldScan := autoRefresh.scanPending

	if !settling {
		autoRefresh.scanPending = false
		autoRefresh.burstOpen = false
	}

	autoRefresh.timer = nil
	autoRefresh.mu.Unlock()

	// onFire is a test seam. The debounce unit tests install it to count fires
	// without a fully wired Handler, and it is nil in production. This path
	// returns before the settle logic, so those tests observe only the
	// debounce.
	if autoRefresh.onFire != nil {
		if settling || shouldScan {
			autoRefresh.onFire()
		}

		return
	}

	if settling {
		// The timer that fired belonged to a settle session, so run its next
		// check.
		h.runSettleCheckLocked()

		return
	}

	if !shouldScan {
		return
	}

	h.runAutoRefreshScanLocked()
	h.beginSettleSessionLocked()
}

// resumeHeldRefreshLocked releases a refresh that was paused while the user was
// typing, now that the typing interaction has ended. Caller must hold h.mu.
func (h *Handler) resumeHeldRefreshLocked() {
	h.autoRefresh.resumeHeld()
}

// stopAutoRefreshTimer cancels any pending refresh timer and clears the burst
// and settle state. Safe to call under the mode lock.
func (h *Handler) stopAutoRefreshTimer() {
	h.autoRefresh.stopAll()
}

// runAutoRefreshScanLocked performs one auto-refresh scan, for a debounce
// fire or a settle check. It runs on the timer goroutine with the mode lock
// held and re-enters the same path a manual refresh uses, so only one scan
// runs at a time. That path keeps the overlay up while it re-draws, so
// re-drawing an unchanged hint set does not flash. The mid-typing hold happens
// earlier, in fireHintRefresh, so by the time this runs the user is not
// typing. Caller must hold h.mu.
func (h *Handler) runAutoRefreshScanLocked() {
	if h.appState.CurrentMode() != domain.ModeHints {
		return
	}

	if !h.autoRefreshEnabledLocked() {
		return
	}

	h.logger.Debug("auto-refresh: observed change")

	// A refresh keeps every session setting it is not handed explicitly. The
	// role/text filters and the search flag must still be passed, because the
	// activation consumes them directly. The filters feed the scan, and the
	// search flag re-opens the search input that the refresh closes.
	filterRoles := h.hints.Context.FilterRoles()
	filterTextContains := h.hints.Context.FilterTextContains()
	startWithSearch := h.hints.Context.StartWithSearch()

	// Mark this as the auto-refresh's own scan so a transient empty scan (a
	// page mid-load) keeps the session alive instead of exiting, and so the
	// debounce gate is skipped for this re-entry.
	h.autoRefreshScanning = true
	defer func() { h.autoRefreshScanning = false }()

	h.activateHintModeInternal(
		nil,
		nil,
		nil,
		filterRoles,
		filterTextContains,
		&startWithSearch,
		nil,
		nil,
		nil,
		nil,
	)
}

// RefreshAfterScroll re-scans hints after a neru-issued scroll, so the labels
// track the content the scroll just moved. It re-enters hints the same way a
// bound `hints` re-launch does. When auto-refresh is enabled, the debounce
// merges it with any refresh already under way. When auto-refresh is off, it
// scans immediately. Outside an active hints session it does nothing. Callers
// must not hold the mode lock.
func (h *Handler) RefreshAfterScroll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeHints {
		return
	}

	if h.hints == nil || h.hints.Context == nil {
		return
	}

	// A refresh keeps every session setting it is not handed explicitly. The
	// filters and the search flag must still be passed, because the activation
	// consumes them directly.
	filterRoles := h.hints.Context.FilterRoles()
	filterTextContains := h.hints.Context.FilterTextContains()
	startWithSearch := h.hints.Context.StartWithSearch()

	h.activateHintModeInternal(
		nil,
		nil,
		nil,
		filterRoles,
		filterTextContains,
		&startWithSearch,
		nil,
		nil,
		nil,
		nil,
	)
}

// updateAutoRefreshObservers points the AX observer at the focused app when
// hints auto_refresh is enabled, and tears the observer down when it is not.
// It runs on every hints activation, fresh or refresh, with the mode lock
// held. That way the observer follows focus. When a refresh runs after the
// front app changed (for example a Cmd+Tab passthrough), the observer moves to
// the new app. Watch also retries a pid whose previous attempt failed, and
// does nothing when the pid is already watched.
func (h *Handler) updateAutoRefreshObservers(bundleID string) {
	if !h.autoRefreshEnabledLocked() {
		h.disarmAutoRefreshObservers()

		return
	}

	autoRefresh := h.config.Hints.AutoRefresh

	h.autoRefresh.setTiming(time.Duration(autoRefresh.MinRefreshDelayMs) * time.Millisecond)

	pid := focusedAppPID(bundleID)
	if pid <= 0 {
		// When the focused pid cannot be resolved (for example a bundle-ID
		// lookup timing out under load), the existing observer stays up,
		// because tearing it down would leave nothing to drive refreshes.
		h.logger.Debug("auto_refresh: focused pid unresolved, keeping current observers",
			zap.String("bundle_id", bundleID))

		return
	}

	h.logger.Debug("auto_refresh: watching focused app",
		zap.Int("pid", pid), zap.String("bundle_id", bundleID))

	axobserver.Watch(pid)
}

// disarmAutoRefreshObservers tears down the observer and cancels any pending
// refresh. It runs on hints-mode exit with the mode lock held, and does
// nothing when nothing is armed. Unwatch joins the observer thread before the
// timer stops, so any in-flight observer callback finishes first and cannot
// arm a new timer after the teardown.
func (h *Handler) disarmAutoRefreshObservers() {
	axobserver.Unwatch()
	h.stopAutoRefreshTimer()
}

// isMidTyping reports whether the user has typed a partial selection that a
// refresh would discard: an active text search with a query, a partly-typed
// hint label, or a multi-match search confirm awaiting its label.
func (h *Handler) isMidTyping() bool {
	if h.hints == nil || h.hints.Context == nil {
		return false
	}

	if h.hintLabelSelectionPending {
		return true
	}

	if h.hintSearchInProgress() {
		return true
	}

	if router := h.hints.Context.Router(); router != nil && router.CurrentInput() != "" {
		return true
	}

	return false
}

// hintSearchInProgress reports whether the user is typing a hint text-search
// query. A manual refresh that lands during one is held until the search ends, so
// the query survives the keystrokes. Caller must hold h.mu.
func (h *Handler) hintSearchInProgress() bool {
	return h.hints != nil && h.hints.Context != nil &&
		h.hints.Context.SearchActive() && h.hints.Context.SearchQuery() != ""
}

// publishEmptyHintsLocked replaces the current hint set with an empty one and
// resumes indicator polling. It runs when a refresh transiently finds no
// hints, and it keeps the session consistent while empty: the overlay redraws
// blank, the router has nothing stale to match, and the mode indicator keeps
// running. The next observed change repopulates the hints. Caller must hold
// the mode lock.
func (h *Handler) publishEmptyHintsLocked() {
	if h.hints == nil || h.hints.Context == nil || h.hints.Context.Manager() == nil {
		return
	}

	if err := h.hints.Context.SetHints(domainHint.NewCollection(nil)); err != nil {
		h.logger.Error("Failed to clear hints on auto-refresh keep-alive", zap.Error(err))
	}

	h.startIndicatorPolling(domain.ModeHints)
}

// focusedAppPID resolves the process id of the app owning bundleID, or 0.
//
// Known limitation: the bundle-ID string is all the activation path carries, so
// when two running processes share a bundle ID this can resolve the one that is
// not focused, and the observer then watches the wrong process. Fixing that
// needs the system port to expose the focused app's pid directly.
func focusedAppPID(bundleID string) int {
	if bundleID == "" {
		return 0
	}

	element := accessibility.ApplicationByBundleID(bundleID)
	if element == nil {
		return 0
	}

	info, err := element.Info()
	if err != nil {
		return 0
	}

	return info.PID()
}
