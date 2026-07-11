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
	// hints.auto_refresh.debounce_ms is unset or non-positive.
	defaultAutoRefreshDebounce = 150 * time.Millisecond

	// autoRefreshMaxWaitFactor derives the max-wait cap from the debounce window.
	// Under sustained UI churn the trailing timer keeps getting pushed back; the
	// cap forces a refresh once a burst has run this many debounce windows, so a
	// continuously-animating app still refreshes at a bounded rate rather than
	// never settling.
	autoRefreshMaxWaitFactor = 4
)

// autoRefreshEnabled reports whether the live config enables hints auto-refresh.
// Caller must hold h.mu (it reads h.config).
func (h *Handler) autoRefreshEnabled() bool {
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

// holdRefreshLocked pauses the debounce while the user is typing: it marks a
// refresh owed and cancels the timer, so nothing scans and nothing fires on an
// interval. The owed refresh is released by resumeHeldRefreshLocked when a search
// interaction ends (confirm or cancel), or, for any other way typing ends, by the
// next observer event or manual refresh. firstDirty is restarted so a subsequent
// burst settles from now rather than firing immediately against a stale max-wait.
// Caller must hold autoRefreshMu.
func (h *Handler) holdRefreshLocked() {
	h.autoRefreshBurstOpen = true
	h.autoRefreshScanPending = true
	h.autoRefreshBurstStart = time.Now()

	if h.autoRefreshTimer != nil {
		h.autoRefreshTimer.Stop()
		h.autoRefreshTimer = nil
	}
}

// beginHintRefresh is the debounce gate every manual in-hints refresh passes
// through while auto_refresh is enabled — a --repeat re-entry, a passthrough
// re-scan, a bound `hints` re-launch, a cycle-hint re-scan. It returns true for
// the leading edge — the first refresh in an idle burst, or a burst whose timer
// was paused for typing — which scans immediately so a manual refresh is never
// delayed, and false when a refresh is already in flight (a single trailing scan
// is (re)scheduled instead), or when it is held for a search in progress.
//
// A refresh that lands while a search query is being typed is held and released
// when the search ends (confirm or cancel), so the query and its filtered hint
// set are not swapped out from under the keystrokes. A pending hint-label
// selection is deliberately NOT held: the --repeat re-scan that runs right after
// a label is chosen would otherwise freeze the overlay on the stale filter with
// no event left to release it. Runs under the mode lock and takes only the leaf
// autoRefreshMu.
func (h *Handler) beginHintRefresh() bool {
	if h.hintSearchInProgress() {
		h.autoRefreshMu.Lock()
		h.holdRefreshLocked()
		h.autoRefreshMu.Unlock()

		return false
	}

	h.autoRefreshMu.Lock()
	defer h.autoRefreshMu.Unlock()

	now := time.Now()

	// An idle burst, or a burst whose timer was paused for typing (open but with
	// no timer), is the leading edge: scan now and (re)open the window. Scanning
	// here also absorbs any refresh the observer path held while typing.
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

// fireHintRefresh runs when the burst timer expires. While the user is
// mid-typing it holds the refresh instead of scanning, so the change waits for
// typing to end rather than being dropped or polled on an interval. Otherwise it
// closes the burst and, if any refresh arrived during the window (a lone manual
// leading edge does not set this), performs the deferred scan. It acquires the
// mode lock first and only then the leaf mutex, so it never holds autoRefreshMu
// while taking h.mu.
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
	shouldScan := h.autoRefreshScanPending
	h.autoRefreshScanPending = false
	h.autoRefreshBurstOpen = false
	h.autoRefreshTimer = nil
	h.autoRefreshMu.Unlock()

	if !shouldScan {
		return
	}

	// Test seam: exercised by the debounce unit tests to count fires without a
	// fully wired Handler. Nil in production.
	if h.autoRefreshOnFire != nil {
		h.autoRefreshOnFire()

		return
	}

	h.observerDrivenRefreshLocked()
}

// resumeHeldRefreshLocked releases a refresh that the debounce paused while the
// user was typing, re-arming it to fire now that the typing interaction has
// ended. It is a cheap no-op when nothing is owed. Caller must hold h.mu.
func (h *Handler) resumeHeldRefreshLocked() {
	h.autoRefreshMu.Lock()
	defer h.autoRefreshMu.Unlock()

	if h.autoRefreshScanPending && h.autoRefreshTimer == nil {
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

	if h.autoRefreshTimer != nil {
		h.autoRefreshTimer.Stop()
		h.autoRefreshTimer = nil
	}
}

// observerDrivenRefreshLocked performs a debounced auto-refresh scan. It runs on
// the burst-timer goroutine with the mode lock held. It re-scans through the
// same path a manual refresh uses, so it is single-flight under the mode lock.
// The mid-typing hold is applied earlier, in fireHintRefresh, so by the time
// this runs the user is not mid-typing. Caller must hold h.mu.
func (h *Handler) observerDrivenRefreshLocked() {
	if h.appState.CurrentMode() != domain.ModeHints {
		return
	}

	if !h.autoRefreshEnabled() {
		return
	}

	h.logger.Debug("auto-refresh: observed change")

	filterRoles := h.hints.Context.FilterRoles()
	filterTextContains := h.hints.Context.FilterTextContains()
	startWithSearch := h.hints.Context.StartWithSearch()
	strategyOverride := h.hints.Context.StrategyOverride()
	labelDirectionOverride := h.hints.Context.LabelDirectionOverride()

	// Mark this as the debounced auto-refresh fire so a transient empty scan (a
	// page mid-load) keeps the session alive instead of exiting, and so the
	// debounce gate is skipped for this re-entry.
	h.hintRefreshFiring = true
	defer func() { h.hintRefreshFiring = false }()

	h.activateHintModeInternal(
		nil,
		nil,
		filterRoles,
		filterTextContains,
		&startWithSearch,
		&strategyOverride,
		&labelDirectionOverride,
	)
}

// updateAutoRefreshObservers points the AX observer at the focused app when
// hints auto_refresh is enabled, and tears it down when it is not. It runs on
// every hints activation (fresh or refresh) while the mode lock is held, so the
// observer follows focus: a refresh after the front app changed (for example a
// Cmd+Tab passthrough) re-targets it. Reconcile is the single source of truth —
// it re-targets a changed pid, retries a pid whose previous arm failed, and is a
// no-op for a pid already armed with the same mask.
func (h *Handler) updateAutoRefreshObservers(bundleID string) {
	if h.observerMgr == nil {
		return
	}

	if !h.autoRefreshEnabled() {
		h.disarmAutoRefreshObservers()

		return
	}

	autoRefresh := h.config.Hints.AutoRefresh

	h.setAutoRefreshTiming(time.Duration(autoRefresh.DebounceMs) * time.Millisecond)

	mask, err := axobserver.MaskFromNames(autoRefresh.AllowedNotifications)
	if err != nil {
		// The config validator rejects unknown names, so this only trips if an
		// unvalidated config reached the handler. Disarm rather than observe an
		// unexpected notification set.
		h.logger.Warn("auto_refresh: unknown notification name, disarming observers",
			zap.Error(err))
		h.disarmAutoRefreshObservers()

		return
	}

	if mask == 0 {
		// An empty allowed_notifications list means nothing to watch.
		h.disarmAutoRefreshObservers()

		return
	}

	pid := focusedAppPID(bundleID)
	if pid <= 0 {
		// A transient failure to resolve the focused pid (for example a bundle-ID
		// lookup timeout under load) keeps the existing observer rather than
		// tearing down the only refresh driver.
		h.logger.Debug("auto_refresh: focused pid unresolved, keeping current observers",
			zap.String("bundle_id", bundleID))

		return
	}

	h.logger.Debug("auto_refresh: reconciling observer on focused app",
		zap.Int("pid", pid), zap.String("bundle_id", bundleID), zap.Uint32("mask", uint32(mask)))

	h.observerMgr.Reconcile([]axobserver.Target{{PID: pid, Mask: mask}})
}

// disarmAutoRefreshObservers tears down every observer and cancels a pending
// refresh. It runs on hints-mode exit while the mode lock is held, and is a
// cheap no-op when nothing is armed. DisarmAll joins the observer run-loop
// thread before the timer is stopped, so an in-flight observer callback (which
// only touches the leaf mutex) completes first and nothing can arm a new timer
// afterward.
func (h *Handler) disarmAutoRefreshObservers() {
	if h.observerMgr == nil {
		return
	}

	h.observerMgr.DisarmAll()
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
