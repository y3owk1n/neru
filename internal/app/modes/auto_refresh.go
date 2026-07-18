package modes

import (
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

// initAutoRefresh wires the observer manager and seeds the debounce timing, so
// a hints activation can point the observer at the focused app. Nothing is
// armed until a hints session runs with hints.auto_refresh enabled.
func (h *Handler) initAutoRefresh() {
	h.autoRefreshDebounce = defaultAutoRefreshDebounce
	h.autoRefreshMaxWait = defaultAutoRefreshDebounce * autoRefreshMaxWaitFactor
	h.observerMgr = axobserver.New(func(int) { h.onObserverChange() }, h.logger)
}

func (h *Handler) autoRefreshEnabledLocked() bool {
	return h.config != nil && h.config.Hints.AutoRefresh.Enabled
}

// setAutoRefreshTiming snapshots the debounce window (and the derived max-wait
// cap) from the live config so the observer callback thread can read them under
// the leaf mutex without touching the mode lock. Called on every hints
// activation while auto_refresh is enabled.
func (h *Handler) setAutoRefreshTiming(debounce time.Duration) {
	if debounce <= 0 {
		debounce = defaultAutoRefreshDebounce
	}

	h.autoRefreshMu.Lock()
	h.autoRefreshDebounce = debounce
	h.autoRefreshMaxWait = debounce * autoRefreshMaxWaitFactor
	h.autoRefreshMu.Unlock()
}

// holdRefreshLocked pauses a pending refresh while the user is typing: it cancels
// the timer so nothing scans on an interval. A paused settle keeps its backoff
// state so it resumes where it left off; otherwise a debounce refresh is marked
// owed. Either is released by resumeHeldRefreshLocked when a search interaction
// ends (confirm or cancel), or, for any other way typing ends, by the next
// observer event or manual refresh. Caller must hold autoRefreshMu.
func (h *Handler) holdRefreshLocked() {
	if h.autoRefreshTimer != nil {
		h.autoRefreshTimer.Stop()
		h.autoRefreshTimer = nil
	}

	if h.autoRefreshSettling {
		return
	}

	h.autoRefreshBurstOpen = true
	h.autoRefreshScanPending = true
	h.autoRefreshBurstStart = time.Now()
}

// admitHintRefresh is the debounce gate for every manual in-hints refresh while
// auto_refresh is enabled: a --repeat re-entry, a passthrough re-scan, a bound
// `hints` re-launch, a cycle-hint re-scan. It returns true when the caller
// should scan right now. That is the leading edge, meaning the first refresh in
// an idle burst, or one whose timer was paused for typing, so a manual refresh
// is never delayed. It returns false when a scan is already in flight, in which
// case it schedules a single trailing scan instead, or when it holds the
// refresh because a search is in progress.
//
// A refresh that lands while a search query is being typed is held and released
// when the search ends (confirm or cancel), so the query and its filtered hint
// set survive the keystrokes. A pending hint-label selection is not held: the
// --repeat re-scan that runs right after a label is chosen must proceed, or the
// overlay would freeze on the stale filter with no event left to release it.
// Runs under the mode lock and takes only the leaf autoRefreshMu.
func (h *Handler) admitHintRefresh() bool {
	if h.hintSearchInProgress() {
		h.autoRefreshMu.Lock()
		h.holdRefreshLocked()
		h.autoRefreshMu.Unlock()

		return false
	}

	h.autoRefreshMu.Lock()
	defer h.autoRefreshMu.Unlock()

	now := time.Now()

	// A manual or --repeat refresh outranks a running settle loop. Stop the
	// settle so this refresh scans immediately below instead of waiting behind
	// the settle's next re-check, which is what would freeze the overlay on the
	// hint the user just selected. Stopping it also clears the settle timer, so
	// the leading-edge check below sees no active timer and admits this refresh.
	// A later observed change starts a fresh settle from the new scan.
	if h.autoRefreshSettling {
		h.stopSettleInnerLocked()
	}

	// The leading edge is an idle burst, or a burst whose timer was paused for
	// typing (marked open but with no timer running). Either way, scan now and
	// (re)open the debounce window. This scan also picks up any refresh the
	// observer path deferred while the user was typing.
	if !h.autoRefreshBurstOpen || h.autoRefreshTimer == nil {
		h.autoRefreshBurstOpen = true
		h.autoRefreshBurstStart = now
		h.autoRefreshScanPending = false
		h.armAutoRefreshTimerLocked(now)

		return true
	}

	h.autoRefreshScanPending = true
	h.armAutoRefreshTimerLocked(now)

	return false
}

// onObserverChange records a UI change reported by an AX observer. It runs on
// the observer callback thread and must never take the mode lock — a teardown
// holds the mode lock while joining that thread — so it only updates the leaf
// debounce state and arms the timer. The observer cannot scan inline, so even
// the leading edge of an observer-only burst is delivered by the timer, which
// also collapses the flurry of notifications a single page load emits into one
// scan.
func (h *Handler) onObserverChange() {
	h.autoRefreshMu.Lock()
	defer h.autoRefreshMu.Unlock()

	now := time.Now()

	// A change during the settle loop means the page is still moving: restart the
	// backoff dense so the next re-scan is soon, but leave the scan-count/window
	// ceilings untouched so a continuously-changing page still winds down. Stop
	// once the window ceiling is reached rather than re-arming forever.
	if h.autoRefreshSettling {
		if now.Sub(h.settleStart) >= autoRefreshSettleMaxDuration {
			h.stopSettleInnerLocked()

			return
		}

		h.settleInterval = autoRefreshSettleBaseInterval
		h.settleStableAtCap = 0
		h.armSettleTimerLocked(autoRefreshSettleBaseInterval)

		return
	}

	if !h.autoRefreshBurstOpen {
		h.autoRefreshBurstOpen = true
		h.autoRefreshBurstStart = now
	}

	h.autoRefreshScanPending = true
	h.armAutoRefreshTimerLocked(now)
}

// armAutoRefreshTimerLocked (re)arms the burst timer, firing after the debounce
// window but never later than the max-wait cap measured from the first change in
// the burst. Caller must hold autoRefreshMu.
func (h *Handler) armAutoRefreshTimerLocked(now time.Time) {
	delay := h.autoRefreshDebounce
	if delay <= 0 {
		delay = defaultAutoRefreshDebounce
	}

	if remaining := h.autoRefreshBurstStart.Add(h.autoRefreshMaxWait).Sub(now); remaining < delay {
		delay = remaining
	}

	if delay < 0 {
		delay = 0
	}

	if h.autoRefreshTimer != nil {
		h.autoRefreshTimer.Stop()
	}

	h.autoRefreshTimer = time.AfterFunc(delay, h.fireHintRefresh)
}

// fireHintRefresh runs when the timer expires, for both a debounce burst and a
// settle re-check. While the user is mid-typing it holds instead of scanning, so
// the change waits for typing to end rather than being dropped or polled on an
// interval. A debounce fire performs the deferred scan and enters the settle
// loop; a settle fire runs one backoff step. It acquires the mode lock first and
// only then the leaf mutex, so it never holds autoRefreshMu while taking h.mu.
func (h *Handler) fireHintRefresh() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.isMidTyping() {
		h.autoRefreshMu.Lock()
		h.holdRefreshLocked()
		h.autoRefreshMu.Unlock()

		return
	}

	h.autoRefreshMu.Lock()
	settling := h.autoRefreshSettling
	shouldScan := h.autoRefreshScanPending
	if !settling {
		h.autoRefreshScanPending = false
		h.autoRefreshBurstOpen = false
	}
	h.autoRefreshTimer = nil
	h.autoRefreshMu.Unlock()

	// Test seam: the debounce unit tests count fires without a fully wired Handler.
	// Nil in production. It never enters the settle loop, so those tests only
	// observe the debounce behavior.
	if h.autoRefreshOnFire != nil {
		if settling || shouldScan {
			h.autoRefreshOnFire()
		}

		return
	}

	if settling {
		h.settleFireLocked()

		return
	}

	if !shouldScan {
		return
	}

	h.observerDrivenRefreshLocked()
	h.beginSettleLocked()
}

// resumeHeldRefreshLocked releases a refresh that was paused while the user was
// typing, re-arming it now that the typing interaction has ended. A paused settle
// resumes at its current backoff interval; otherwise an owed debounce refresh is
// re-armed. It is a cheap no-op when nothing is owed or a timer is already
// running. Caller must hold h.mu.
func (h *Handler) resumeHeldRefreshLocked() {
	h.autoRefreshMu.Lock()
	defer h.autoRefreshMu.Unlock()

	if h.autoRefreshTimer != nil {
		return
	}

	if h.autoRefreshSettling {
		h.armSettleTimerLocked(h.settleInterval)

		return
	}

	if h.autoRefreshScanPending {
		h.armAutoRefreshTimerLocked(time.Now())
	}
}

// stopAutoRefreshTimer cancels any pending burst timer and closes the burst. It
// takes the leaf mutex and is safe to call under the mode lock.
func (h *Handler) stopAutoRefreshTimer() {
	h.autoRefreshMu.Lock()
	defer h.autoRefreshMu.Unlock()

	h.autoRefreshBurstOpen = false
	h.autoRefreshScanPending = false
	h.autoRefreshSettling = false
	h.settleInterval = 0
	h.settleStableAtCap = 0
	h.settleScanCount = 0
	h.lastAppliedFingerprint = 0

	if h.autoRefreshTimer != nil {
		h.autoRefreshTimer.Stop()
		h.autoRefreshTimer = nil
	}
}

// observerDrivenRefreshLocked performs one auto-refresh scan (a debounce fire or
// a settle re-check). It runs on the timer goroutine with the mode lock held and
// re-scans through the same path a manual refresh uses, so it is single-flight
// under the mode lock. That path skips blanking the overlay on a refresh (the
// flicker fix), so re-drawing the same hints does not flash. The mid-typing hold
// is applied earlier, in fireHintRefresh, so by the time this runs the user is
// not mid-typing. Caller must hold h.mu.
func (h *Handler) observerDrivenRefreshLocked() {
	if h.appState.CurrentMode() != domain.ModeHints {
		return
	}

	if !h.autoRefreshEnabledLocked() {
		return
	}

	h.logger.Debug("auto-refresh: observed change")

	filterRoles := h.hints.Context.FilterRoles()
	filterTextContains := h.hints.Context.FilterTextContains()
	startWithSearch := h.hints.Context.StartWithSearch()
	hideOnEmptySearch := h.hints.Context.HideOnEmptySearch()
	strategyOverride := h.hints.Context.StrategyOverride()
	labelDirectionOverride := h.hints.Context.LabelDirectionOverride()
	splitWord := h.hints.Context.SplitWord()

	// Mark this as the debounced auto-refresh fire so a transient empty scan (a
	// page mid-load) keeps the session alive instead of exiting, and so the
	// debounce gate is skipped for this re-entry.
	h.hintRefreshFiring = true
	defer func() { h.hintRefreshFiring = false }()

	h.activateHintModeInternal(
		nil,
		nil,
		nil,
		filterRoles,
		filterTextContains,
		&startWithSearch,
		&hideOnEmptySearch,
		&strategyOverride,
		&labelDirectionOverride,
		&splitWord,
	)
}

// updateAutoRefreshObservers points the AX observer at the focused app when
// hints auto_refresh is enabled, and tears it down when it is not. It runs on
// every hints activation (fresh or refresh) while the mode lock is held, so the
// observer follows focus: a refresh after the front app changed (for example a
// Cmd+Tab passthrough) re-targets it. Watch re-targets a changed pid, retries a
// pid whose previous arm failed (a failed arm leaves the slot empty), and is a
// no-op for the pid already watched.
func (h *Handler) updateAutoRefreshObservers(bundleID string) {
	if h.observerMgr == nil {
		return
	}

	if !h.autoRefreshEnabledLocked() {
		h.disarmAutoRefreshObservers()

		return
	}

	autoRefresh := h.config.Hints.AutoRefresh

	h.setAutoRefreshTiming(time.Duration(autoRefresh.MinRefreshDelayMs) * time.Millisecond)

	pid := focusedAppPID(bundleID)
	if pid <= 0 {
		// A transient failure to resolve the focused pid (for example a bundle-ID
		// lookup timeout under load) keeps the existing observer rather than
		// tearing down the only refresh driver.
		h.logger.Debug("auto_refresh: focused pid unresolved, keeping current observers",
			zap.String("bundle_id", bundleID))

		return
	}

	h.logger.Debug("auto_refresh: watching focused app",
		zap.Int("pid", pid), zap.String("bundle_id", bundleID))

	h.observerMgr.Watch(pid)
}

// disarmAutoRefreshObservers tears down the observer and cancels a pending
// refresh. It runs on hints-mode exit while the mode lock is held, and is a
// cheap no-op when nothing is armed. Unwatch joins the observer run-loop
// thread before the timer is stopped, so an in-flight observer callback (which
// only touches the leaf mutex) completes first and nothing can arm a new timer
// afterward.
func (h *Handler) disarmAutoRefreshObservers() {
	if h.observerMgr == nil {
		return
	}

	h.observerMgr.Unwatch()
	h.stopAutoRefreshTimer()
}

// isMidTyping reports whether the user has typed a partial selection that a
// refresh would discard: an active text search with a query, or a partly-typed
// hint label.
func (h *Handler) isMidTyping() bool {
	if h.hints == nil || h.hints.Context == nil {
		return false
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
// resumes indicator polling, keeping the session consistent when a refresh
// transiently finds no hints: the overlay is redrawn blank, the router matches
// nothing (no stale routing), and the mode indicator keeps running. The caller
// must hold the mode lock. The next observed change repopulates the hints.
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
