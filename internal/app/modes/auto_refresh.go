package modes

import (
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
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

	// Settle-backoff parameters. After an observer-driven scan the page may still
	// be rendering with no further AX notification, so the settle loop re-scans at
	// a widening interval: base, then ×growth each stable step (1.6, as 8/5), until
	// it reaches the cap. It resets to base whenever the hint set changes, and
	// stops only after two consecutive stable scans at the cap. The scan-count and
	// window ceilings bound a page that keeps changing so it cannot re-scan forever.
	autoRefreshSettleBase       = 250 * time.Millisecond
	autoRefreshSettleCap        = 5 * time.Second
	autoRefreshSettleGrowthNum  = 8
	autoRefreshSettleGrowthDen  = 5
	autoRefreshSettleStopStable = 2
	autoRefreshSettleMaxScans   = 25
	autoRefreshSettleMaxWindow  = 30 * time.Second
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

	// A manual or --repeat refresh takes priority over a running settle loop: end
	// the settle so this scans now (the leading edge below) rather than being
	// deferred behind a pending settle re-check, which would freeze the overlay on
	// a hint selection. Ending it clears the settle timer, so the leading-edge test
	// passes even if a stale burst flag lingered. A later observer change re-seeds
	// the settle from the fresh scan.
	if h.autoRefreshSettling {
		h.stopSettleInnerLocked()
	}

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

	// A change during the settle loop means the page is still moving: restart the
	// backoff dense so the next re-scan is soon, but leave the scan-count/window
	// ceilings untouched so a continuously-changing page still winds down. Stop
	// once the window ceiling is reached rather than re-arming forever.
	if h.autoRefreshSettling {
		if now.Sub(h.settleStart) >= autoRefreshSettleMaxWindow {
			h.stopSettleLocked()

			return
		}

		h.settleInterval = autoRefreshSettleBase
		h.settleStableAtCap = 0
		h.armSettleTimerLocked(autoRefreshSettleBase)

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

	if !h.autoRefreshEnabled() {
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

// beginSettleLocked starts the settle loop after an observer-driven scan. Web
// content often renders in waves with no AX notification for the later ones, so
// one scan can catch a page mid-render. It records the applied hint set's
// fingerprint as the baseline and arms the first re-scan at the base interval.
// Caller must hold h.mu.
func (h *Handler) beginSettleLocked() {
	if h.appState.CurrentMode() != domain.ModeHints || !h.autoRefreshEnabled() {
		return
	}

	fingerprint := h.fingerprintHintsLocked()

	h.autoRefreshMu.Lock()
	defer h.autoRefreshMu.Unlock()

	h.autoRefreshSettling = true
	h.lastAppliedFingerprint = fingerprint
	h.settleInterval = autoRefreshSettleBase
	h.settleStableAtCap = 0
	h.settleScanCount = 0
	h.settleStart = time.Now()
	h.armSettleTimerLocked(autoRefreshSettleBase)
}

// settleFireLocked runs one backoff step: re-scan, compare the new hint set to
// what is applied, then either reset the interval dense (it changed), grow the
// interval (stable), or stop. The re-scan redraws only what changed via the
// refresh path's incremental draw, so a stable step is invisible. Caller holds
// h.mu.
func (h *Handler) settleFireLocked() {
	stillHints := h.appState.CurrentMode() == domain.ModeHints && h.autoRefreshEnabled()

	if stillHints {
		h.observerDrivenRefreshLocked()
		stillHints = h.appState.CurrentMode() == domain.ModeHints
	}

	var fingerprint uint64
	if stillHints {
		fingerprint = h.fingerprintHintsLocked()
	}

	h.autoRefreshMu.Lock()
	defer h.autoRefreshMu.Unlock()

	if !stillHints {
		h.stopSettleInnerLocked()

		return
	}

	changed := fingerprint != h.lastAppliedFingerprint
	h.lastAppliedFingerprint = fingerprint

	if h.advanceSettleLocked(changed) {
		h.stopSettleInnerLocked()

		return
	}

	h.armSettleTimerLocked(h.settleInterval)
}

// advanceSettleLocked updates the backoff after a settle scan and reports whether
// the loop should stop. A change restarts the interval dense; a stable scan grows
// it, or counts toward the stop once it is at the cap. Caller holds autoRefreshMu.
func (h *Handler) advanceSettleLocked(changed bool) bool {
	h.settleScanCount++

	switch {
	case changed:
		h.settleInterval = autoRefreshSettleBase
		h.settleStableAtCap = 0
	case h.settleInterval >= autoRefreshSettleCap:
		h.settleStableAtCap++
	default:
		h.settleInterval = nextSettleInterval(h.settleInterval)
	}

	return settleShouldStop(h.settleInterval, h.settleStableAtCap, h.settleScanCount, time.Since(h.settleStart))
}

// stopSettleLocked ends the settle loop and clears its state. Caller holds h.mu
// but not autoRefreshMu.
func (h *Handler) stopSettleLocked() {
	h.autoRefreshMu.Lock()
	h.stopSettleInnerLocked()
	h.autoRefreshMu.Unlock()
}

// stopSettleInnerLocked ends the settle loop and clears its state. Caller holds
// autoRefreshMu.
func (h *Handler) stopSettleInnerLocked() {
	h.autoRefreshSettling = false
	h.settleInterval = 0
	h.settleStableAtCap = 0
	h.settleScanCount = 0

	if h.autoRefreshTimer != nil {
		h.autoRefreshTimer.Stop()
		h.autoRefreshTimer = nil
	}
}

// armSettleTimerLocked (re)arms the timer to fire the next settle re-check after
// delay. Caller holds autoRefreshMu.
func (h *Handler) armSettleTimerLocked(delay time.Duration) {
	if delay < 0 {
		delay = 0
	}

	if h.autoRefreshTimer != nil {
		h.autoRefreshTimer.Stop()
	}

	h.autoRefreshTimer = time.AfterFunc(delay, h.fireHintRefresh)
}

// nextSettleInterval grows the backoff interval by the growth factor (8/5 = 1.6),
// clamped to the cap.
func nextSettleInterval(current time.Duration) time.Duration {
	next := current * autoRefreshSettleGrowthNum / autoRefreshSettleGrowthDen
	if next > autoRefreshSettleCap {
		return autoRefreshSettleCap
	}

	return next
}

// settleShouldStop reports whether the settle loop has finished: two consecutive
// stable scans once the interval reached the cap, or either global ceiling (total
// scans, or elapsed since the sequence began) reached.
func settleShouldStop(interval time.Duration, stableAtCap, scanCount int, elapsed time.Duration) bool {
	if interval >= autoRefreshSettleCap && stableAtCap >= autoRefreshSettleStopStable {
		return true
	}

	if scanCount >= autoRefreshSettleMaxScans {
		return true
	}

	return elapsed >= autoRefreshSettleMaxWindow
}

// fingerprintHintsLocked hashes the current hint set (element bounds and role) so
// two scans can be compared for equality without walking the accessibility tree
// again. Caller must hold h.mu.
func (h *Handler) fingerprintHintsLocked() uint64 {
	if h.hints == nil || h.hints.Context == nil {
		return 0
	}

	collection := h.hints.Context.Hints()
	if collection == nil {
		return 0
	}

	const (
		fnvOffset uint64 = 14695981039346656037
		fnvPrime  uint64 = 1099511628211
	)

	hash := fnvOffset
	mix := func(v uint64) {
		hash = (hash ^ v) * fnvPrime
	}

	hints := collection.All()
	mix(uint64(len(hints)))

	for _, hint := range hints {
		bounds := hint.Bounds()
		mix(uint64(uint32(bounds.Min.X)))
		mix(uint64(uint32(bounds.Min.Y)))
		mix(uint64(uint32(bounds.Max.X)))
		mix(uint64(uint32(bounds.Max.Y)))

		for _, char := range hint.Element().Role() {
			mix(uint64(char))
		}
	}

	return hash
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

	if !h.autoRefreshEnabled() {
		h.disarmAutoRefreshObservers()

		return
	}

	autoRefresh := h.config.Hints.AutoRefresh

	h.setAutoRefreshTiming(time.Duration(autoRefresh.DebounceMs) * time.Millisecond)

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
