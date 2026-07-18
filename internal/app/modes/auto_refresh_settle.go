package modes

import (
	"time"

	"github.com/y3owk1n/neru/internal/core/domain"
)

const (
	// Settle-backoff parameters. After an observer-driven scan the page may still
	// be rendering with no further AX notification, so the settle loop keeps
	// re-scanning on its own at a widening interval. The interval starts at the
	// base interval and grows by the growth ratio (the numerator over the
	// denominator) after each scan that finds the hint set unchanged, up to the
	// max interval. Any change resets the interval to the base. The loop stops
	// after enough consecutive unchanged scans at the max interval, or when it
	// reaches either ceiling (total scans or elapsed duration), so a page that
	// never stops changing still winds down.
	autoRefreshSettleBaseInterval      = 250 * time.Millisecond
	autoRefreshSettleMaxInterval       = 5 * time.Second
	autoRefreshSettleGrowthNumerator   = 8
	autoRefreshSettleGrowthDenominator = 5
	autoRefreshSettleStableScansToStop = 2
	autoRefreshSettleMaxScans          = 25
	autoRefreshSettleMaxDuration       = 30 * time.Second
)

// beginSettleLocked starts the settle loop after an observer-driven scan. Web
// content often renders in waves with no AX notification for the later ones, so
// one scan can catch a page mid-render. It records the applied hint set's
// fingerprint as the baseline and arms the first re-scan at the base interval.
// Caller must hold h.mu.
func (h *Handler) beginSettleLocked() {
	if h.appState.CurrentMode() != domain.ModeHints || !h.autoRefreshEnabledLocked() {
		return
	}

	fingerprint := h.fingerprintHintsLocked()

	h.autoRefreshMu.Lock()
	defer h.autoRefreshMu.Unlock()

	h.autoRefreshSettling = true
	h.lastAppliedFingerprint = fingerprint
	h.settleInterval = autoRefreshSettleBaseInterval
	h.settleStableAtCap = 0
	h.settleScanCount = 0
	h.settleStart = time.Now()
	h.armSettleTimerLocked(autoRefreshSettleBaseInterval)
}

// settleFireLocked runs one backoff step: re-scan, compare the new hint set to
// what is applied, then either reset the interval to the base (it changed), grow
// the interval (stable), or stop. The re-scan redraws only what changed via the
// refresh path's incremental draw, so a stable step is invisible. Caller holds
// h.mu.
func (h *Handler) settleFireLocked() {
	stillHints := h.appState.CurrentMode() == domain.ModeHints && h.autoRefreshEnabledLocked()

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
// the loop should stop. A change resets the interval to the base; a stable scan
// grows it, or counts toward the stop once it is at the max. Caller holds
// autoRefreshMu.
func (h *Handler) advanceSettleLocked(changed bool) bool {
	h.settleScanCount++

	switch {
	case changed:
		h.settleInterval = autoRefreshSettleBaseInterval
		h.settleStableAtCap = 0
	case h.settleInterval >= autoRefreshSettleMaxInterval:
		h.settleStableAtCap++
	default:
		h.settleInterval = nextSettleInterval(h.settleInterval)
	}

	return settleShouldStop(
		h.settleInterval,
		h.settleStableAtCap,
		h.settleScanCount,
		time.Since(h.settleStart),
	)
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

// nextSettleInterval grows the backoff interval by the growth ratio, clamped to
// the max interval.
func nextSettleInterval(current time.Duration) time.Duration {
	next := current * autoRefreshSettleGrowthNumerator / autoRefreshSettleGrowthDenominator
	if next > autoRefreshSettleMaxInterval {
		return autoRefreshSettleMaxInterval
	}

	return next
}

// settleShouldStop reports whether the settle loop has finished: enough
// consecutive stable scans once the interval reached the max, or either global
// ceiling (total scans, or elapsed since the sequence began) reached.
func settleShouldStop(
	interval time.Duration,
	stableAtCap, scanCount int,
	elapsed time.Duration,
) bool {
	if interval >= autoRefreshSettleMaxInterval &&
		stableAtCap >= autoRefreshSettleStableScansToStop {
		return true
	}

	if scanCount >= autoRefreshSettleMaxScans {
		return true
	}

	return elapsed >= autoRefreshSettleMaxDuration
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
