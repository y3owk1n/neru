package modes

import (
	"image"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	domainhint "github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/domain/state"
)

// newDebounceHarness builds a Handler with only the auto-refresh debounce state
// wired, plus a fire counter installed on the test seam, so the
// leading/trailing/max-wait logic can be exercised without a fully constructed
// Handler. The seam short-circuits fireHintRefresh before it would take the mode
// lock, so counting is lock-free and deadlock-free.
func newDebounceHarness(debounce, maxWait time.Duration) (*Handler, *int32, chan struct{}) {
	var count int32

	fired := make(chan struct{}, 32)

	handler := &Handler{
		logger:              zap.NewNop(),
		autoRefreshDebounce: debounce,
		autoRefreshMaxWait:  maxWait,
	}
	handler.autoRefreshOnFire = func() {
		atomic.AddInt32(&count, 1)

		select {
		case fired <- struct{}{}:
		default:
		}
	}

	return handler, &count, fired
}

func TestAdmitHintRefreshLoneLeadingDoesNotFireTrailing(t *testing.T) {
	handler, count, _ := newDebounceHarness(20*time.Millisecond, 200*time.Millisecond)

	if !handler.admitHintRefresh() {
		t.Fatal("the first refresh in an idle burst must be the leading edge (true)")
	}

	// No further request arrives, so the burst timer must close the window
	// without a trailing scan — the leading caller already scanned inline.
	time.Sleep(90 * time.Millisecond)

	if got := atomic.LoadInt32(count); got != 0 {
		t.Fatalf("a lone leading refresh fired %d trailing scans, want 0", got)
	}
}

func TestAdmitHintRefreshSecondRequestFiresOneTrailing(t *testing.T) {
	handler, count, fired := newDebounceHarness(20*time.Millisecond, 200*time.Millisecond)

	if !handler.admitHintRefresh() {
		t.Fatal("the first refresh must be the leading edge")
	}

	if handler.admitHintRefresh() {
		t.Fatal("a refresh during an open burst must defer (false), not lead")
	}

	select {
	case <-fired:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("a deferred refresh never fired its trailing scan")
	}

	// Exactly one trailing scan: the two requests collapse into it.
	time.Sleep(90 * time.Millisecond)

	if got := atomic.LoadInt32(count); got != 1 {
		t.Fatalf("burst fired %d trailing scans, want exactly 1", got)
	}
}

func TestObserverChangeBurstCoalescesToOneFire(t *testing.T) {
	handler, count, fired := newDebounceHarness(30*time.Millisecond, 500*time.Millisecond)

	// A page load emits many notifications in quick succession; they must
	// collapse into a single scan.
	for range 6 {
		handler.onObserverChange()
		time.Sleep(5 * time.Millisecond)
	}

	select {
	case <-fired:
	case <-time.After(400 * time.Millisecond):
		t.Fatal("an observer burst never fired")
	}

	time.Sleep(120 * time.Millisecond)

	if got := atomic.LoadInt32(count); got != 1 {
		t.Fatalf("observer burst fired %d scans, want 1 (should coalesce)", got)
	}
}

func TestAutoRefreshMaxWaitForcesFireUnderContinuousChurn(t *testing.T) {
	handler, _, fired := newDebounceHarness(40*time.Millisecond, 100*time.Millisecond)

	quit := make(chan struct{})
	defer close(quit)

	// Notifications every 20ms keep resetting the 40ms debounce, so debounce
	// alone would never fire. The 100ms max-wait must force a scan anyway.
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				handler.onObserverChange()
			case <-quit:
				return
			}
		}
	}()

	select {
	case <-fired:
	case <-time.After(400 * time.Millisecond):
		t.Fatal("max-wait did not force a scan under continuous churn")
	}
}

func TestStopAutoRefreshTimerCancelsPending(t *testing.T) {
	handler, count, _ := newDebounceHarness(40*time.Millisecond, 500*time.Millisecond)

	handler.onObserverChange()
	handler.stopAutoRefreshTimer()

	time.Sleep(120 * time.Millisecond)

	if got := atomic.LoadInt32(count); got != 0 {
		t.Fatalf("a stopped timer fired %d scans, want 0", got)
	}
}

// newRefreshHandler builds a Handler wired enough to run observerDrivenRefreshLocked
// through its early-return guards: an appState in the given mode, a config that
// enables or disables auto_refresh, and a hints context. The debounce fields and
// the fire seam are set so any re-arm or fire is observable. It is deliberately
// not wired for an actual scan, so the tests below only exercise paths that
// return before activateHintModeInternal.
func newRefreshHandler(mode domain.Mode, enabled bool) (*Handler, *int32) {
	appState := state.NewAppState()
	appState.SetMode(mode)

	var count int32

	handler := &Handler{
		logger:   zap.NewNop(),
		appState: appState,
		config: &configpkg.Config{
			Hints: configpkg.HintsConfig{
				AutoRefresh: configpkg.HintsAutoRefresh{Enabled: enabled},
			},
		},
		hints: &components.HintsComponent{
			Context: &hintscomponent.Context{},
		},
		autoRefreshDebounce: 20 * time.Millisecond,
		autoRefreshMaxWait:  200 * time.Millisecond,
	}
	handler.autoRefreshOnFire = func() { atomic.AddInt32(&count, 1) }

	return handler, &count
}

func (h *Handler) autoRefreshOwed() (owed, armed bool) {
	h.autoRefreshMu.Lock()
	defer h.autoRefreshMu.Unlock()

	return h.autoRefreshScanPending, h.autoRefreshTimer != nil
}

func TestFireHintRefreshHoldsWhileTypingThenResumes(t *testing.T) {
	handler, count := newRefreshHandler(domain.ModeHints, true)
	handler.hints.Context.SetSearchActive(true)
	handler.hints.Context.SetSearchQuery("qu")

	// A refresh is owed, as onObserverChange would have left it before the timer
	// fired.
	handler.autoRefreshMu.Lock()
	handler.autoRefreshBurstOpen = true
	handler.autoRefreshScanPending = true
	handler.autoRefreshMu.Unlock()

	// The burst timer fires while the user is mid-typing: it must hold, not scan,
	// and must not leave a timer running (no interval polling).
	handler.fireHintRefresh()

	if got := atomic.LoadInt32(count); got != 0 {
		t.Fatalf("a mid-typing fire scanned %d times, want 0 (must hold)", got)
	}

	if owed, armed := handler.autoRefreshOwed(); !owed || armed {
		t.Fatalf(
			"after a mid-typing hold: owed=%v armed=%v, want owed=true armed=false",
			owed,
			armed,
		)
	}

	// Nothing fires on its own while typing continues.
	time.Sleep(80 * time.Millisecond)

	if got := atomic.LoadInt32(count); got != 0 {
		t.Fatalf("a held refresh fired %d scans while still typing, want 0", got)
	}

	// The typing interaction ends; the search-end resume releases the held
	// refresh, which now scans because the user is no longer typing.
	handler.hints.Context.SetSearchActive(false)
	handler.hints.Context.SetSearchQuery("")

	handler.mu.Lock()
	handler.resumeHeldRefreshLocked()
	handler.mu.Unlock()

	deadline := time.After(400 * time.Millisecond)

	for atomic.LoadInt32(count) == 0 {
		select {
		case <-deadline:
			t.Fatal("resume did not fire the held refresh after typing ended")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func TestObserverDrivenRefreshSkipsOutsideHintsMode(t *testing.T) {
	handler, count := newRefreshHandler(domain.ModeIdle, true)

	handler.mu.Lock()
	handler.observerDrivenRefreshLocked()
	handler.mu.Unlock()

	time.Sleep(40 * time.Millisecond)

	if got := atomic.LoadInt32(count); got != 0 {
		t.Fatalf("a refresh outside hints mode fired %d scans, want 0", got)
	}
}

func TestObserverDrivenRefreshSkipsWhenDisabled(t *testing.T) {
	handler, count := newRefreshHandler(domain.ModeHints, false)

	handler.mu.Lock()
	handler.observerDrivenRefreshLocked()
	handler.mu.Unlock()

	time.Sleep(40 * time.Millisecond)

	if got := atomic.LoadInt32(count); got != 0 {
		t.Fatalf("a refresh with auto_refresh disabled fired %d scans, want 0", got)
	}
}

func TestAdmitHintRefreshProceedsWithPendingLabel(t *testing.T) {
	handler, _ := newRefreshHandler(domain.ModeHints, true)

	// A partly-typed hint label makes isMidTyping report true, but the manual
	// --repeat re-scan that runs right after a label is selected must still
	// proceed, or the overlay freezes on the stale label filter with no event left
	// to release it.
	handler.mu.Lock()
	manager := domainhint.NewManager(handler.logger, &handler.mu)
	handler.hints.Context.SetManager(manager)

	elem, _ := element.NewElement("t", image.Rect(0, 0, 10, 10), element.RoleButton)
	collection := domainhint.NewCollection([]*domainhint.Interface{mustNewModeHint("AA", elem)})
	setErr := handler.hints.Context.SetHints(collection)
	handler.hints.Context.SetRouter(domainhint.NewRouter(manager, handler.logger))
	handler.mu.Unlock()

	if setErr != nil {
		t.Fatalf("SetHints: %v", setErr)
	}

	handler.mu.Lock()
	_, routeErr := handler.hints.Context.Router().RouteKey("A")
	handler.mu.Unlock()

	if routeErr != nil {
		t.Fatalf("RouteKey: %v", routeErr)
	}

	if !handler.isMidTyping() {
		t.Fatal("precondition: a partly-typed label should report mid-typing")
	}

	if !handler.admitHintRefresh() {
		t.Fatal(
			"admitHintRefresh held a refresh on a pending label; it must proceed (leading edge)",
		)
	}
}

func TestAdmitHintRefreshHoldsWhileSearching(t *testing.T) {
	handler, _ := newRefreshHandler(domain.ModeHints, true)
	handler.hints.Context.SetSearchActive(true)
	handler.hints.Context.SetSearchQuery("qu")

	// A refresh during an active search must be held, not scanned, so the typed
	// query and its filtered hint set survive; it is released when the search ends.
	if handler.admitHintRefresh() {
		t.Fatal("admitHintRefresh proceeded during an active search; it must hold (false)")
	}

	if owed, armed := handler.autoRefreshOwed(); !owed || armed {
		t.Fatalf("after a search hold: owed=%v armed=%v, want owed=true armed=false", owed, armed)
	}
}

func TestIsMidTyping(t *testing.T) {
	handler := &Handler{
		logger: zap.NewNop(),
		hints: &components.HintsComponent{
			Context: &hintscomponent.Context{},
		},
	}

	if handler.isMidTyping() {
		t.Fatal("a fresh hints context should not report mid-typing")
	}

	// A text search with a query is mid-typing.
	handler.hints.Context.SetSearchActive(true)
	handler.hints.Context.SetSearchQuery("sub")

	if !handler.isMidTyping() {
		t.Fatal("an active search with a query should report mid-typing")
	}

	// An active search with an empty query is not.
	handler.hints.Context.SetSearchQuery("")

	if handler.isMidTyping() {
		t.Fatal("an active search with an empty query should not report mid-typing")
	}

	handler.hints.Context.SetSearchActive(false)

	// A partly-typed hint label is mid-typing.
	handler.mu.Lock()
	manager := domainhint.NewManager(handler.logger, &handler.mu)
	handler.hints.Context.SetManager(manager)

	elem, _ := element.NewElement("t", image.Rect(0, 0, 10, 10), element.RoleButton)
	collection := domainhint.NewCollection([]*domainhint.Interface{mustNewModeHint("AA", elem)})
	setErr := handler.hints.Context.SetHints(collection)
	handler.hints.Context.SetRouter(domainhint.NewRouter(manager, handler.logger))
	handler.mu.Unlock()

	if setErr != nil {
		t.Fatalf("SetHints: %v", setErr)
	}

	if handler.isMidTyping() {
		t.Fatal("no typed input should not report mid-typing")
	}

	// RouteKey mutates hint state, which requires the handler lock held (the
	// same lock the auto-refresh fire holds when it consults isMidTyping).
	handler.mu.Lock()
	_, routeErr := handler.hints.Context.Router().RouteKey("A")
	handler.mu.Unlock()

	if routeErr != nil {
		t.Fatalf("RouteKey: %v", routeErr)
	}

	if !handler.isMidTyping() {
		t.Fatal("a partly-typed hint label should report mid-typing")
	}
}

// setSettleHints installs a hint collection of n distinct elements on the
// handler, so the settle-recheck fingerprint has something to hash.
func setSettleHints(t *testing.T, handler *Handler, n int) {
	t.Helper()

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if handler.hints.Context.Manager() == nil {
		handler.hints.Context.SetManager(domainhint.NewManager(handler.logger, &handler.mu))
	}

	hints := make([]*domainhint.Interface, 0, n)
	for i := range n {
		label := string(rune('a' + i))

		elem, err := element.NewElement(
			element.ID(label),
			image.Rect(i*10, i*10, i*10+10, i*10+10),
			element.RoleButton,
		)
		if err != nil {
			t.Fatalf("NewElement: %v", err)
		}

		hints = append(hints, mustNewModeHint(label, elem))
	}

	if err := handler.hints.Context.SetHints(domainhint.NewCollection(hints)); err != nil {
		t.Fatalf("SetHints: %v", err)
	}
}

func (h *Handler) advanceSettle(changed bool) bool {
	h.autoRefreshMu.Lock()
	defer h.autoRefreshMu.Unlock()

	return h.advanceSettleLocked(changed)
}

func (h *Handler) settleSnapshot() (interval time.Duration, stableAtCap, scanCount int) {
	h.autoRefreshMu.Lock()
	defer h.autoRefreshMu.Unlock()

	return h.settleInterval, h.settleStableAtCap, h.settleScanCount
}

func (h *Handler) seedSettle(interval time.Duration, scanCount int) {
	h.autoRefreshMu.Lock()
	defer h.autoRefreshMu.Unlock()

	h.autoRefreshSettling = true
	h.settleInterval = interval
	h.settleStableAtCap = 0
	h.settleScanCount = scanCount
	h.settleStart = time.Now()
}

func TestObserverChangePastSettleWindowStopsWithoutDeadlock(t *testing.T) {
	handler, _ := newRefreshHandler(domain.ModeHints, true)

	handler.seedSettle(autoRefreshSettleBaseInterval, 0)

	// Backdate the settle so this change lands past the window ceiling, which
	// must end the loop.
	handler.autoRefreshMu.Lock()
	handler.settleStart = time.Now().Add(-autoRefreshSettleMaxDuration - time.Second)
	handler.autoRefreshMu.Unlock()

	done := make(chan struct{})

	go func() {
		handler.onObserverChange()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("onObserverChange did not return while ending a settle past its window")
	}

	handler.autoRefreshMu.Lock()
	settling := handler.autoRefreshSettling
	handler.autoRefreshMu.Unlock()

	if settling {
		t.Fatal("a change past the settle window should end the settle loop")
	}
}

func TestFingerprintHintsReflectsHintSet(t *testing.T) {
	handler, _ := newRefreshHandler(domain.ModeHints, true)

	handler.mu.Lock()
	empty := handler.fingerprintHintsLocked()
	handler.mu.Unlock()

	setSettleHints(t, handler, 2)

	handler.mu.Lock()
	two := handler.fingerprintHintsLocked()
	twoAgain := handler.fingerprintHintsLocked()
	handler.mu.Unlock()

	setSettleHints(t, handler, 3)

	handler.mu.Lock()
	three := handler.fingerprintHintsLocked()
	handler.mu.Unlock()

	if two == empty {
		t.Error("a non-empty hint set must fingerprint differently from empty")
	}

	if two != twoAgain {
		t.Error("the same hint set must produce the same fingerprint")
	}

	if three == two {
		t.Error("a changed hint set must produce a different fingerprint")
	}
}

func TestNextSettleInterval(t *testing.T) {
	current := autoRefreshSettleBaseInterval

	for range 20 {
		next := nextSettleInterval(current)
		if next == autoRefreshSettleMaxInterval {
			break
		}

		if want := current * autoRefreshSettleGrowthNumerator / autoRefreshSettleGrowthDenominator; next != want {
			t.Fatalf("nextSettleInterval(%v) = %v, want %v", current, next, want)
		}

		if next <= current {
			t.Fatalf("the interval must grow: %v -> %v", current, next)
		}

		current = next
	}

	if got := nextSettleInterval(
		autoRefreshSettleMaxInterval,
	); got != autoRefreshSettleMaxInterval {
		t.Fatalf("at the cap nextSettleInterval = %v, want %v", got, autoRefreshSettleMaxInterval)
	}

	if got := nextSettleInterval(
		2 * autoRefreshSettleMaxInterval,
	); got != autoRefreshSettleMaxInterval {
		t.Fatalf("past the cap nextSettleInterval = %v, want %v", got, autoRefreshSettleMaxInterval)
	}
}

func TestSettleShouldStop(t *testing.T) {
	// Below the cap it never stops on stability, however many stable scans.
	if settleShouldStop(autoRefreshSettleBaseInterval, 5, 1, 0) {
		t.Error("must not stop below the cap on stable scans")
	}

	if settleShouldStop(autoRefreshSettleMaxInterval-time.Millisecond, 5, 1, 0) {
		t.Error("must not stop just under the cap")
	}

	// At the cap it needs two consecutive stable scans.
	if settleShouldStop(autoRefreshSettleMaxInterval, 1, 1, 0) {
		t.Error("one stable scan at the cap must not stop")
	}

	if !settleShouldStop(autoRefreshSettleMaxInterval, autoRefreshSettleStableScansToStop, 1, 0) {
		t.Error("two stable scans at the cap must stop")
	}

	// Either global ceiling stops regardless of interval or stability.
	if !settleShouldStop(autoRefreshSettleBaseInterval, 0, autoRefreshSettleMaxScans, 0) {
		t.Error("the scan-count ceiling must stop")
	}

	if !settleShouldStop(autoRefreshSettleBaseInterval, 0, 1, autoRefreshSettleMaxDuration) {
		t.Error("the window ceiling must stop")
	}
}

func TestAdvanceSettleClimbsResetsAndStops(t *testing.T) {
	handler, _ := newRefreshHandler(domain.ModeHints, true)
	handler.seedSettle(autoRefreshSettleBaseInterval, 0)

	// A stable scan below the cap grows the interval and does not stop.
	if handler.advanceSettle(false) {
		t.Fatal("a stable scan below the cap must not stop")
	}

	if iv, _, count := handler.settleSnapshot(); iv != nextSettleInterval(
		autoRefreshSettleBaseInterval,
	) ||
		count != 1 {
		t.Fatalf("after one stable scan: interval=%v count=%d", iv, count)
	}

	// A change restarts the interval dense but keeps the scan count climbing.
	handler.advanceSettle(false)

	if handler.advanceSettle(true) {
		t.Fatal("a changed scan must not stop")
	}

	if iv, stable, count := handler.settleSnapshot(); iv != autoRefreshSettleBaseInterval ||
		stable != 0 ||
		count != 3 {
		t.Fatalf(
			"after a change: interval=%v stableAtCap=%d count=%d, want base/0/3",
			iv,
			stable,
			count,
		)
	}

	// At the cap, the first stable scan does not stop; the second does.
	handler.seedSettle(autoRefreshSettleMaxInterval, 0)

	if handler.advanceSettle(false) {
		t.Fatal("one stable scan at the cap must not stop")
	}

	if _, stable, _ := handler.settleSnapshot(); stable != 1 {
		t.Fatalf("after one stable-at-cap scan stableAtCap = %d, want 1", stable)
	}

	if !handler.advanceSettle(false) {
		t.Fatal("two stable scans at the cap must stop")
	}
}

func TestAdvanceSettleGlobalScanCapStopsAChangingPage(t *testing.T) {
	handler, _ := newRefreshHandler(domain.ModeHints, true)
	// Seed one scan short of the ceiling; the next scan reaches it.
	handler.seedSettle(autoRefreshSettleBaseInterval, autoRefreshSettleMaxScans-1)

	// Even though the set keeps changing (interval stays dense), the scan-count
	// ceiling must still stop the loop so a live page cannot re-scan forever.
	if !handler.advanceSettle(true) {
		t.Fatal("reaching the scan-count ceiling must stop even while the page keeps changing")
	}
}

func TestAdmitHintRefreshPreemptsSettle(t *testing.T) {
	handler, _ := newRefreshHandler(domain.ModeHints, true)
	// A long debounce keeps the leading-edge timer from firing during the test.
	handler.autoRefreshDebounce = time.Hour
	handler.autoRefreshMaxWait = time.Hour

	defer handler.stopAutoRefreshTimer()

	// A settle loop is running with a pending re-check, as it would be when a hint
	// is selected mid-settle.
	handler.autoRefreshMu.Lock()
	handler.autoRefreshSettling = true
	handler.settleInterval = autoRefreshSettleMaxInterval
	handler.settleStableAtCap = 1
	handler.settleStart = time.Now()
	handler.armSettleTimerLocked(time.Hour)
	handler.autoRefreshMu.Unlock()

	// A manual or --repeat refresh must win: scan now (leading edge, true) rather
	// than be deferred behind the settle, which is what froze the overlay.
	if !handler.admitHintRefresh() {
		t.Fatal("a manual refresh during a settle loop must lead (true), not defer")
	}

	handler.autoRefreshMu.Lock()
	settling := handler.autoRefreshSettling
	handler.autoRefreshMu.Unlock()

	if settling {
		t.Fatal("a manual refresh must end the running settle loop")
	}
}
