package modes

import (
	"hash/fnv"
	"time"

	"github.com/y3owk1n/neru/internal/core/domain"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// observerSelfScanSuppress is how long observer-driven refreshes are muted after
// a no-op scan, to swallow the create/destroy notifications that scan induces in
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

// beginObserverScanWindow starts scan suppression: it raises the scanning flag
// (which drops observer refreshes for the whole scan) and resets the per-scan
// fingerprint state. Called at the top of activateHintModeInternal, under h.mu.
func (h *Handler) beginObserverScanWindow() {
	h.observerScanning.Store(true)
	h.observerScanHasFingerprint = false
}

// recordScanFingerprint stores the fingerprint of the hint set this scan
// produced, so endObserverScanWindow can tell a real change from self-induced
// churn. Called once the final hint set is known, under h.mu.
func (h *Handler) recordScanFingerprint(hints []*domainHint.Interface) {
	h.observerScanFingerprint = fingerprintHints(hints)
	h.observerScanHasFingerprint = true
}

// endObserverScanWindow ends scan suppression. Deferred from
// activateHintModeInternal, so it runs on every exit path, under h.mu.
//
// It opens the short post-scan margin only when the scan produced the same hint
// set as the previous scan (self-induced churn, nothing really changed). A scan
// that changed the hint set caught a real change, so it clears any lingering
// margin and stays hot: the change is often still settling and will post more
// notifications, and dropping those was what made roughly one refresh in five
// miss the fresh hints. A scan that never reached a fingerprint (an early error
// or empty result) leaves the stored fingerprint untouched and opens no margin.
func (h *Handler) endObserverScanWindow() {
	defer h.observerScanning.Store(false)

	if !h.observerScanHasFingerprint {
		return
	}

	changed := h.observerScanFingerprint != h.observerLastFingerprint
	h.observerLastFingerprint = h.observerScanFingerprint

	if changed {
		h.observerSuppressUntil.Store(0)

		return
	}

	h.observerSuppressUntil.Store(time.Now().Add(observerSelfScanSuppress).UnixNano())
}

// fingerprintHints computes an order-independent fingerprint of a hint set from
// each element's stable identity and bounds. Two scans that resolve the same
// elements at the same positions produce the same value; adding, removing, or
// moving an element changes it. Order independence (XOR-combining per-element
// hashes) means a reordered-but-identical set is correctly seen as unchanged.
func fingerprintHints(hints []*domainHint.Interface) uint64 {
	var combined uint64

	for _, hint := range hints {
		el := hint.Element()
		if el == nil {
			continue
		}

		hh := fnv.New64a()
		_, _ = hh.Write([]byte(el.StableID()))

		b := el.Bounds()
		var box [8]byte
		putInt16(box[0:], b.Min.X)
		putInt16(box[2:], b.Min.Y)
		putInt16(box[4:], b.Dx())
		putInt16(box[6:], b.Dy())
		_, _ = hh.Write(box[:])

		combined ^= hh.Sum64()
	}

	// Fold the count in so a set that XORs to the same value with a different
	// number of elements (e.g. a duplicated pair) is still seen as changed.
	return combined ^ (uint64(len(hints)) * 0x9E3779B97F4A7C15)
}

func putInt16(dst []byte, v int) {
	dst[0] = byte(v)
	dst[1] = byte(v >> 8)
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
