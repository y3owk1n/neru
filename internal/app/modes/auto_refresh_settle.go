// Settle recheck: web content often keeps rendering without sending
// accessibility notifications. A scan can catch a page mid-render, or miss part
// of the changes. After each scan initiated by an observer event, a settle
// recheck session re-checks the page on a timer, comparing hint-set
// fingerprints. When nothing changes, it checks again after a growing interval,
// up to a max. When the page changes, it resets the interval duration. The base
// interval duration rises under sustained changes, so a page that never stops
// changing is followed at a bounded rate. The session ends after the interval
// reaches the maximum duration and doesn't detect any changes in a few
// consecutive scans, or when a new accessibility notification hands scheduling
// back to the debounce. State lives on hintAutoRefresh in auto_refresh.go.

package modes

import (
	"time"

	"github.com/y3owk1n/neru/internal/core/domain"
)

const (
	// Pacing for the settle recheck. The wait between checks starts at the base
	// interval and grows by the growth factor. When a check finds nothing
	// changed, the wait grows toward the max interval. When the wait is at the
	// max and a few consecutive checks still find nothing, the session ends.
	// When a check finds changes, the wait resets to the floor. The floor stays
	// at the base interval for the first few changes and then grows by the same
	// factor, up to the floor cap.
	autoRefreshSettleBaseInterval      = 250 * time.Millisecond
	autoRefreshSettleMaxInterval       = 5 * time.Second
	autoRefreshSettleGrowthFactor      = 1.6
	autoRefreshSettleStableScansToStop = 2
	autoRefreshSettleFreeFastChecks    = 4
	autoRefreshSettleFloorCap          = 2 * time.Second
)

// beginSettleSessionLocked starts a settle session after an observer-driven scan,
// since that scan can have caught the page mid-render. It records the applied
// hint set's fingerprint as the baseline and arms the first check at the base
// interval. Caller must hold h.mu.
func (h *Handler) beginSettleSessionLocked() {
	if h.appState.CurrentMode() != domain.ModeHints || !h.autoRefreshEnabledLocked() {
		return
	}

	fingerprint := h.fingerprintHintsLocked()

	autoRefresh := &h.autoRefresh

	autoRefresh.mu.Lock()
	defer autoRefresh.mu.Unlock()

	autoRefresh.settling = true
	autoRefresh.lastFingerprint = fingerprint
	autoRefresh.floor = autoRefreshSettleBaseInterval
	autoRefresh.changedCount = 0
	autoRefresh.interval = autoRefreshSettleBaseInterval
	autoRefresh.stableAtCap = 0
	autoRefresh.armTimerLocked(autoRefreshSettleBaseInterval)
}

// runSettleCheckLocked runs one check of the settle loop. It re-scans the page and
// compares the new hint set to the previous one. When the set changed, the wait
// resets to the floor. When nothing changed, the wait grows, or the session
// ends once it has been at the max for a few unchanged checks. The re-scan
// redraws only the hints that changed, so a check that finds nothing leaves the
// screen untouched. Caller must hold h.mu.
func (h *Handler) runSettleCheckLocked() {
	// The session only runs while hints mode is open and auto-refresh is on.
	stillHints := h.appState.CurrentMode() == domain.ModeHints && h.autoRefreshEnabledLocked()

	if stillHints {
		// Re-scan the page and publish the fresh hint set.
		h.runAutoRefreshScanLocked()
		// The scan can end the session on some failure paths, so check the mode
		// again after the scan.
		stillHints = h.appState.CurrentMode() == domain.ModeHints
	}

	// Hash the freshly published hint set so it can be compared with the hash
	// the previous check recorded.
	var fingerprint uint64
	if stillHints {
		fingerprint = h.fingerprintHintsLocked()
	}

	autoRefresh := &h.autoRefresh

	autoRefresh.mu.Lock()
	defer autoRefresh.mu.Unlock()

	// Stop the session if hints mode ended.
	if !stillHints {
		autoRefresh.stopSettleSessionLocked()

		return
	}

	// If the fingerprint changed, the page has changed since the previous
	// check. The new fingerprint becomes the baseline for the next one.
	changed := fingerprint != autoRefresh.lastFingerprint
	autoRefresh.lastFingerprint = fingerprint

	// Adjust the wait for the next check. The session is finished once the
	// page has stayed unchanged long enough.
	sessionFinished := autoRefresh.advanceSettleLocked(changed)
	if sessionFinished {
		autoRefresh.stopSettleSessionLocked()
	} else {
		// Schedule the next check after the adjusted wait.
		autoRefresh.armTimerLocked(autoRefresh.interval)
	}
}

// advanceSettleLocked updates the wait after a settle check and reports
// whether the session is finished. When the check found changes, the wait
// resets to the floor, and after the first few changes each further change
// also raises the floor. When the check found nothing, the wait grows. When
// the wait is already at the max and enough consecutive checks find nothing,
// the session is finished. Caller must hold mu.
func (ar *hintAutoRefresh) advanceSettleLocked(changed bool) bool {
	switch {
	case changed:
		ar.changedCount++
		if ar.changedCount > autoRefreshSettleFreeFastChecks {
			ar.floor = nextSettleFloor(ar.floor)
		}

		ar.interval = ar.floor
		ar.stableAtCap = 0
	case ar.interval >= autoRefreshSettleMaxInterval:
		ar.stableAtCap++
	default:
		ar.interval = nextSettleInterval(ar.interval)
	}

	// True means the session is finished: the wait has reached the max
	// interval and enough consecutive checks found no changes.
	return ar.interval >= autoRefreshSettleMaxInterval &&
		ar.stableAtCap >= autoRefreshSettleStableScansToStop
}

// stopSettleSessionLocked ends the settle session and clears its state. Bumping the
// timer generation invalidates a fire that already left its timer, so a late
// fire cannot act on the cleared state. Caller must hold mu.
func (ar *hintAutoRefresh) stopSettleSessionLocked() {
	ar.timerGen++
	ar.settling = false
	ar.interval = 0
	ar.stableAtCap = 0
	ar.floor = 0
	ar.changedCount = 0

	if ar.timer != nil {
		ar.timer.Stop()
		ar.timer = nil
	}
}

// nextSettleInterval grows the backoff interval by the growth factor, clamped
// to the max interval.
func nextSettleInterval(current time.Duration) time.Duration {
	next := time.Duration(float64(current) * autoRefreshSettleGrowthFactor)
	if next > autoRefreshSettleMaxInterval {
		return autoRefreshSettleMaxInterval
	}

	return next
}

// nextSettleFloor grows the session floor by the growth factor, clamped to the
// floor cap.
func nextSettleFloor(current time.Duration) time.Duration {
	next := time.Duration(float64(current) * autoRefreshSettleGrowthFactor)
	if next > autoRefreshSettleFloorCap {
		return autoRefreshSettleFloorCap
	}

	return next
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
