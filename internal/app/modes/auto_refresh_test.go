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

	handler := &Handler{logger: zap.NewNop()}
	handler.autoRefresh.fire = handler.fireHintRefresh
	handler.autoRefresh.debounce = debounce
	handler.autoRefresh.maxWait = maxWait
	handler.autoRefresh.onFire = func() {
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
		t.Fatal("the first refresh while the debounce is idle must scan immediately (true)")
	}

	// No further request arrives, so the burst timer must close the window
	// without a trailing scan, because the first caller already scanned inline.
	time.Sleep(90 * time.Millisecond)

	if got := atomic.LoadInt32(count); got != 0 {
		t.Fatalf("a single admitted refresh fired %d trailing scans, want 0", got)
	}
}

func TestAdmitHintRefreshSecondRequestFiresOneTrailing(t *testing.T) {
	handler, count, fired := newDebounceHarness(20*time.Millisecond, 200*time.Millisecond)

	if !handler.admitHintRefresh() {
		t.Fatal("the first refresh must scan immediately")
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
		handler.autoRefresh.onChange()
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
				handler.autoRefresh.onChange()
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

func TestStaleFireLeavesCurrentTimerUntouched(t *testing.T) {
	handler, count, _ := newDebounceHarness(20*time.Millisecond, 200*time.Millisecond)

	// A fire is pending with generation N when a re-arm supersedes it: the
	// superseded fire must not scan, must not consume the owed flags, and must
	// not discard the live timer the re-arm installed.
	liveTimer := time.AfterFunc(10*time.Minute, func() {})
	defer liveTimer.Stop()

	handler.autoRefresh.mu.Lock()
	handler.autoRefresh.burstOpen = true
	handler.autoRefresh.scanPending = true
	handler.autoRefresh.timerGen = 7
	staleGen := handler.autoRefresh.timerGen
	handler.autoRefresh.timerGen++ // the re-arm that superseded the pending fire
	handler.autoRefresh.timer = liveTimer
	handler.autoRefresh.mu.Unlock()

	handler.fireHintRefresh(staleGen)

	if got := atomic.LoadInt32(count); got != 0 {
		t.Fatalf("a stale fire scanned %d times, want 0", got)
	}

	handler.autoRefresh.mu.Lock()
	timer := handler.autoRefresh.timer
	burstOpen := handler.autoRefresh.burstOpen
	scanPending := handler.autoRefresh.scanPending
	handler.autoRefresh.mu.Unlock()

	if timer != liveTimer {
		t.Fatal("a stale fire discarded the live timer")
	}

	if !burstOpen || !scanPending {
		t.Fatalf("a stale fire consumed burst state: burstOpen=%v scanPending=%v, want both true",
			burstOpen, scanPending)
	}
}

func TestStopAutoRefreshTimerCancelsPending(t *testing.T) {
	handler, count, _ := newDebounceHarness(40*time.Millisecond, 500*time.Millisecond)

	handler.autoRefresh.onChange()
	handler.stopAutoRefreshTimer()

	time.Sleep(120 * time.Millisecond)

	if got := atomic.LoadInt32(count); got != 0 {
		t.Fatalf("a stopped timer fired %d scans, want 0", got)
	}
}

// newRefreshHandler builds a Handler wired enough to run runAutoRefreshScanLocked
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
	}
	handler.autoRefresh.fire = handler.fireHintRefresh
	handler.autoRefresh.debounce = 20 * time.Millisecond
	handler.autoRefresh.maxWait = 200 * time.Millisecond
	handler.autoRefresh.onFire = func() { atomic.AddInt32(&count, 1) }

	return handler, &count
}

func (h *Handler) autoRefreshOwed() (owed, armed bool) {
	h.autoRefresh.mu.Lock()
	defer h.autoRefresh.mu.Unlock()

	return h.autoRefresh.scanPending, h.autoRefresh.timer != nil
}

func TestFireHintRefreshHoldsWhileTypingThenResumes(t *testing.T) {
	handler, count := newRefreshHandler(domain.ModeHints, true)
	handler.hints.Context.SetSearchActive(true)
	handler.hints.Context.SetSearchQuery("qu")

	// A refresh is owed, as onObserverChange would have left it before the timer
	// fired.
	handler.autoRefresh.mu.Lock()
	handler.autoRefresh.burstOpen = true
	handler.autoRefresh.scanPending = true
	handler.autoRefresh.mu.Unlock()

	// The burst timer fires while the user is mid-typing: it must hold, not scan,
	// and must not leave a timer running (no interval polling). The direct call
	// passes the current generation so it counts as the tracked timer's fire.
	handler.autoRefresh.mu.Lock()
	gen := handler.autoRefresh.timerGen
	handler.autoRefresh.mu.Unlock()

	handler.fireHintRefresh(gen)

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
	handler.runAutoRefreshScanLocked()
	handler.mu.Unlock()

	time.Sleep(40 * time.Millisecond)

	if got := atomic.LoadInt32(count); got != 0 {
		t.Fatalf("a refresh outside hints mode fired %d scans, want 0", got)
	}
}

func TestObserverDrivenRefreshSkipsWhenDisabled(t *testing.T) {
	handler, count := newRefreshHandler(domain.ModeHints, false)

	handler.mu.Lock()
	handler.runAutoRefreshScanLocked()
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
			"admitHintRefresh must admit a refresh while a label selection is pending",
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
	h.autoRefresh.mu.Lock()
	defer h.autoRefresh.mu.Unlock()

	return h.autoRefresh.advanceSettleLocked(changed)
}

func (h *Handler) settleSnapshot() (interval, floor time.Duration, stableAtCap, changedCount int) {
	h.autoRefresh.mu.Lock()
	defer h.autoRefresh.mu.Unlock()

	return h.autoRefresh.interval, h.autoRefresh.floor, h.autoRefresh.stableAtCap, h.autoRefresh.changedCount
}

func (h *Handler) seedSettle(interval time.Duration) {
	h.autoRefresh.mu.Lock()
	defer h.autoRefresh.mu.Unlock()

	h.autoRefresh.settling = true
	h.autoRefresh.interval = interval
	h.autoRefresh.floor = autoRefreshSettleBaseInterval
	h.autoRefresh.changedCount = 0
	h.autoRefresh.stableAtCap = 0
}

func TestRefreshAfterScrollIsNoOpOutsideHints(t *testing.T) {
	handler, count := newRefreshHandler(domain.ModeIdle, true)

	handler.RefreshAfterScroll()

	if got := atomic.LoadInt32(count); got != 0 {
		t.Fatalf("a scroll refresh outside hints mode scanned %d times, want 0", got)
	}
}

func TestRefreshAfterScrollDefersIntoAnOpenBurst(t *testing.T) {
	handler, _ := newRefreshHandler(domain.ModeHints, true)
	handler.autoRefresh.debounce = time.Hour
	handler.autoRefresh.maxWait = time.Hour

	defer handler.stopAutoRefreshTimer()

	// An auto-refresh burst is open with a live timer, as it is mid page load.
	handler.autoRefresh.mu.Lock()
	handler.autoRefresh.burstOpen = true
	handler.autoRefresh.burstStart = time.Now()
	handler.autoRefresh.armBurstTimerLocked(time.Now())
	handler.autoRefresh.mu.Unlock()

	// The scroll refresh must merge into the burst through the debounce gate: a
	// single trailing scan is owed instead of a second immediate one.
	handler.RefreshAfterScroll()

	handler.autoRefresh.mu.Lock()
	scanPending := handler.autoRefresh.scanPending
	handler.autoRefresh.mu.Unlock()

	if !scanPending {
		t.Fatal("a scroll refresh during an open burst must defer into the trailing scan")
	}
}

func TestObserverChangeInterruptsSettle(t *testing.T) {
	handler, _ := newRefreshHandler(domain.ModeHints, true)

	handler.seedSettle(autoRefreshSettleBaseInterval)

	done := make(chan struct{})

	go func() {
		handler.autoRefresh.onChange()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("onObserverChange did not return while interrupting a settle")
	}

	handler.autoRefresh.mu.Lock()
	settling := handler.autoRefresh.settling
	burstOpen := handler.autoRefresh.burstOpen
	scanPending := handler.autoRefresh.scanPending
	timerArmed := handler.autoRefresh.timer != nil
	handler.autoRefresh.mu.Unlock()

	if settling {
		t.Fatal("a change during the settle must end the settle session")
	}

	if !burstOpen || !scanPending || !timerArmed {
		t.Fatalf("a change during the settle must open a debounce burst: "+
			"burstOpen=%v scanPending=%v timerArmed=%v, want all true",
			burstOpen, scanPending, timerArmed)
	}

	handler.stopAutoRefreshTimer()
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

		if want := time.Duration(float64(current) * autoRefreshSettleGrowthFactor); next != want {
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

func TestNextSettleFloor(t *testing.T) {
	current := autoRefreshSettleBaseInterval

	for range 20 {
		next := nextSettleFloor(current)
		if next == autoRefreshSettleFloorCap {
			break
		}

		if want := time.Duration(float64(current) * autoRefreshSettleGrowthFactor); next != want {
			t.Fatalf("nextSettleFloor(%v) = %v, want %v", current, next, want)
		}

		current = next
	}

	if got := nextSettleFloor(autoRefreshSettleFloorCap); got != autoRefreshSettleFloorCap {
		t.Fatalf("at the floor cap nextSettleFloor = %v, want %v", got, autoRefreshSettleFloorCap)
	}
}

func TestAdvanceSettleClimbsResetsAndStops(t *testing.T) {
	handler, _ := newRefreshHandler(domain.ModeHints, true)
	handler.seedSettle(autoRefreshSettleBaseInterval)

	// A check that finds no changes below the max grows the interval and does
	// not stop the session.
	if handler.advanceSettle(false) {
		t.Fatal("a no-change check below the max must not stop the session")
	}

	if interval, _, _, _ := handler.settleSnapshot(); interval != nextSettleInterval(
		autoRefreshSettleBaseInterval,
	) {
		t.Fatalf("after one no-change check: interval=%v", interval)
	}

	// A change resets the interval to the floor and never stops the session.
	handler.advanceSettle(false)

	if handler.advanceSettle(true) {
		t.Fatal("a check that finds changes must not stop the session")
	}

	if interval, floor, stable, changed := handler.settleSnapshot(); interval != autoRefreshSettleBaseInterval ||
		floor != autoRefreshSettleBaseInterval ||
		stable != 0 ||
		changed != 1 {
		t.Fatalf(
			"after a change: interval=%v floor=%v stableAtCap=%d changedCount=%d, want base/base/0/1",
			interval,
			floor,
			stable,
			changed,
		)
	}

	// At the max, the first no-change check does not stop the session and the
	// second one does.
	handler.seedSettle(autoRefreshSettleMaxInterval)

	if handler.advanceSettle(false) {
		t.Fatal("one no-change check at the max must not stop the session")
	}

	if _, _, stable, _ := handler.settleSnapshot(); stable != 1 {
		t.Fatalf("after one no-change check at the max stableAtCap = %d, want 1", stable)
	}

	if !handler.advanceSettle(false) {
		t.Fatal("two no-change checks at the max must stop the session")
	}
}

func TestAdvanceSettleFloorRisesUnderSustainedChanges(t *testing.T) {
	handler, _ := newRefreshHandler(domain.ModeHints, true)
	handler.seedSettle(autoRefreshSettleBaseInterval)

	// The free fast checks keep the floor at the base.
	for check := range autoRefreshSettleFreeFastChecks {
		if handler.advanceSettle(true) {
			t.Fatalf("changed check %d must not stop", check+1)
		}

		if interval, floor, _, _ := handler.settleSnapshot(); interval != autoRefreshSettleBaseInterval ||
			floor != autoRefreshSettleBaseInterval {
			t.Fatalf(
				"free check %d: interval=%v floor=%v, want base/base",
				check+1,
				interval,
				floor,
			)
		}
	}

	// Each further changed check raises the floor toward its cap, and the
	// interval follows the floor.
	wantFloor := autoRefreshSettleBaseInterval

	for check := range 20 {
		if handler.advanceSettle(true) {
			t.Fatalf("sustained change %d must not stop the session", check+1)
		}

		wantFloor = nextSettleFloor(wantFloor)

		interval, floor, _, _ := handler.settleSnapshot()
		if floor != wantFloor || interval != wantFloor {
			t.Fatalf("sustained change %d: interval=%v floor=%v, want %v",
				check+1, interval, floor, wantFloor)
		}
	}

	if _, floor, _, _ := handler.settleSnapshot(); floor != autoRefreshSettleFloorCap {
		t.Fatalf("sustained changes must converge the floor to the cap; floor=%v", floor)
	}

	// A change at the max resets the interval to the raised floor, and the
	// session must then see two fresh no-change checks at the max before it
	// ends.
	handler.autoRefresh.mu.Lock()
	handler.autoRefresh.interval = autoRefreshSettleMaxInterval
	handler.autoRefresh.stableAtCap = 1
	handler.autoRefresh.mu.Unlock()

	if handler.advanceSettle(true) {
		t.Fatal("a change at the max must not stop the session")
	}

	if interval, _, stable, _ := handler.settleSnapshot(); interval != autoRefreshSettleFloorCap ||
		stable != 0 {
		t.Fatalf("a change at the max must reset to the floor: interval=%v stableAtCap=%d",
			interval, stable)
	}

	// Ending the session discards the raised floor. A fresh session starts back
	// at the base.
	handler.autoRefresh.mu.Lock()
	handler.autoRefresh.stopSettleSessionLocked()
	floorAfterStop := handler.autoRefresh.floor
	handler.autoRefresh.mu.Unlock()

	if floorAfterStop != 0 {
		t.Fatalf("ending the session must clear the floor; floor=%v", floorAfterStop)
	}

	handler.seedSettle(autoRefreshSettleBaseInterval)

	if _, floor, _, _ := handler.settleSnapshot(); floor != autoRefreshSettleBaseInterval {
		t.Fatalf("a fresh session must start at the base floor; floor=%v", floor)
	}
}

func TestAdmitHintRefreshPreemptsSettle(t *testing.T) {
	handler, _ := newRefreshHandler(domain.ModeHints, true)
	// A long debounce keeps the burst timer from firing during the test.
	handler.autoRefresh.debounce = time.Hour
	handler.autoRefresh.maxWait = time.Hour

	defer handler.stopAutoRefreshTimer()

	// A settle loop is running with a pending re-check, as it would be when a hint
	// is selected mid-settle.
	handler.autoRefresh.mu.Lock()
	handler.autoRefresh.settling = true
	handler.autoRefresh.interval = autoRefreshSettleMaxInterval
	handler.autoRefresh.stableAtCap = 1
	handler.autoRefresh.armTimerLocked(time.Hour)
	handler.autoRefresh.mu.Unlock()

	// A manual or --repeat refresh must win and scan now (true). If it deferred
	// behind the settle, the overlay would freeze on the just-selected hint.
	if !handler.admitHintRefresh() {
		t.Fatal("a manual refresh during a settle session must scan now (true), not defer")
	}

	handler.autoRefresh.mu.Lock()
	settling := handler.autoRefresh.settling
	handler.autoRefresh.mu.Unlock()

	if settling {
		t.Fatal("a manual refresh must end the running settle loop")
	}
}
