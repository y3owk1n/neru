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

func TestBeginHintRefreshLoneLeadingDoesNotFireTrailing(t *testing.T) {
	handler, count, _ := newDebounceHarness(20*time.Millisecond, 200*time.Millisecond)

	if !handler.beginHintRefresh() {
		t.Fatal("the first refresh in an idle burst must be the leading edge (true)")
	}

	// No further request arrives, so the burst timer must close the window
	// without a trailing scan — the leading caller already scanned inline.
	time.Sleep(90 * time.Millisecond)

	if got := atomic.LoadInt32(count); got != 0 {
		t.Fatalf("a lone leading refresh fired %d trailing scans, want 0", got)
	}
}

func TestBeginHintRefreshSecondRequestFiresOneTrailing(t *testing.T) {
	handler, count, fired := newDebounceHarness(20*time.Millisecond, 200*time.Millisecond)

	if !handler.beginHintRefresh() {
		t.Fatal("the first refresh must be the leading edge")
	}

	if handler.beginHintRefresh() {
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
		t.Fatalf("after a mid-typing hold: owed=%v armed=%v, want owed=true armed=false", owed, armed)
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

func TestBeginHintRefreshProceedsWithPendingLabel(t *testing.T) {
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

	if !handler.beginHintRefresh() {
		t.Fatal("beginHintRefresh held a refresh on a pending label; it must proceed (leading edge)")
	}
}

func TestBeginHintRefreshHoldsWhileSearching(t *testing.T) {
	handler, _ := newRefreshHandler(domain.ModeHints, true)
	handler.hints.Context.SetSearchActive(true)
	handler.hints.Context.SetSearchQuery("qu")

	// A refresh during an active search must be held, not scanned, so the typed
	// query and its filtered hint set survive; it is released when the search ends.
	if handler.beginHintRefresh() {
		t.Fatal("beginHintRefresh proceeded during an active search; it must hold (false)")
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
