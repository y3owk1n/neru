package modes

import (
	"time"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// observerSelfScanSuppress is how long observer-driven refreshes are muted after
// a scan starts, to swallow the create/destroy notifications a scan induces in
// some apps (which would otherwise loop into another refresh).
const observerSelfScanSuppress = 200 * time.Millisecond

// ObserverController is the subset of the push-based AX observer manager that the
// Handler drives. It is satisfied by *axobserver.Manager; the Handler depends on
// this interface so the modes package stays free of platform infrastructure.
type ObserverController interface {
	// Reconcile sets the processes to observe for the current hint scan.
	Reconcile(targets []ports.ObservationTarget)
	// DisarmAll tears down every observer (called when hints mode exits).
	DisarmAll()
	// Close shuts the observer down for good (called at app teardown).
	Close()
}

// SetObserverController wires the push-based change observer. Called once at
// startup, before hints mode is ever entered.
func (h *Handler) SetObserverController(controller ObserverController) {
	h.observers = controller
}

// autoRefreshEnabled reports whether push auto-refresh is configured on and an
// observer is wired.
func (h *Handler) autoRefreshEnabled() bool {
	return h.observers != nil && h.config != nil && h.config.Hints.AutoRefresh.Enabled
}

// RequestObserverRefresh is the observer manager's change sink: a non-stale
// accessibility notification arrived, so ask the coordinator for a coalesced
// refresh. Runs on the observer run-loop thread and must not block.
func (h *Handler) RequestObserverRefresh() {
	// Drop notifications while a scan is running, and for a short margin after it:
	// a scan makes some apps churn their own AX elements throughout, and those
	// self-induced notifications must not feed back into another refresh.
	if h.observerScanning.Load() {
		return
	}

	if until := h.observerSuppressUntil.Load(); until != 0 && time.Now().UnixNano() < until {
		return
	}

	if h.refreshCoordinator != nil {
		h.refreshCoordinator.Request()
	}
}

// endObserverScanWindow ends scan suppression: it opens a short post-scan margin
// (to catch notifications an app posts just after neru finishes reading) and
// then clears the scanning flag. Deferred from activateHintModeInternal.
func (h *Handler) endObserverScanWindow() {
	h.observerSuppressUntil.Store(time.Now().Add(observerSelfScanSuppress).UnixNano())
	h.observerScanning.Store(false)
}

// observerDrivenRefresh performs a coalesced, observer-triggered refresh. It runs
// on a timer goroutine, so it takes the handler lock and re-checks that hints
// mode is still active before re-scanning.
func (h *Handler) observerDrivenRefresh() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeHints {
		return
	}

	if h.hints == nil || h.hints.Context == nil {
		return
	}

	h.logger.Debug("observer-driven hint refresh")

	// Re-read the session's filter/strategy context so an observer-driven refresh
	// preserves custom roles, text filters, search, and overrides, exactly as the
	// modifier-passthrough refresh does. activateHintModeInternal detects it is a
	// refresh from the current mode and re-scans without changing mode or the tap.
	filterRoles := h.hints.Context.FilterRoles()
	filterTextContains := h.hints.Context.FilterTextContains()
	startWithSearch := h.hints.Context.StartWithSearch()
	strategyOverride := h.hints.Context.StrategyOverride()
	labelDirectionOverride := h.hints.Context.LabelDirectionOverride()

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

// isMidSelection reports whether the user is part-way through choosing a hint (a
// partially typed label, or an active text search), so an auto-refresh should be
// deferred rather than swap the hint set out from under them. Runs on a timer
// goroutine; takes the handler lock.
func (h *Handler) isMidSelection() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.hints == nil || h.hints.Context == nil {
		return false
	}

	if h.hints.Context.SearchActive() {
		return true
	}

	manager := h.hints.Context.Manager()

	return manager != nil && manager.CurrentInput() != ""
}

// reconcileObserversLocked recomputes and applies the observation target set for
// the current scan. The caller holds h.mu. No-op when auto-refresh is off.
func (h *Handler) reconcileObserversLocked(bundleID, strategy string) {
	if !h.autoRefreshEnabled() {
		return
	}

	targets := h.hintService.ObservationTargets(h.ctx, bundleID, strategy)
	h.observers.Reconcile(targets)
}

// ShutdownAutoRefresh stops the refresh coordinator and closes the observer
// controller. Called once at app teardown.
func (h *Handler) ShutdownAutoRefresh() {
	if h.refreshCoordinator != nil {
		h.refreshCoordinator.Stop()
	}

	if h.observers != nil {
		h.observers.Close()
	}
}

// syncObservers is called on every mode transition (from setAppModeLocked). It is
// the single disarm site: leaving hints mode tears down all observers, which
// catches every exit path plus SetModeIdle and any future mode-changing path.
func (h *Handler) syncObservers(mode domain.Mode) {
	if h.observers == nil {
		return
	}

	if mode != domain.ModeHints {
		h.observers.DisarmAll()
	}
}
