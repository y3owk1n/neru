package modes

import (
	"context"
	"image"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// syncOrderSystem is a SystemPort that also implements ports.CursorSynchronizer
// and records the order in which the cursor sync and the ScreenBounds read
// happen. On Wayland ScreenBounds answers from a cursor cache the sync
// refreshes, so a read before the sync selects a stale monitor (#1279).
type syncOrderSystem struct {
	portmocks.MockSystemPort

	mu     sync.Mutex
	events []string
}

var _ ports.CursorSynchronizer = (*syncOrderSystem)(nil)

const (
	eventCursorSync   = "sync"
	eventScreenBounds = "screenBounds"
)

func (s *syncOrderSystem) SyncCursorPosition(context.Context) error {
	s.record(eventCursorSync)

	return nil
}

// record appends an event under the fixture's own lock: the race test drives
// this system from three goroutines at once.
func (s *syncOrderSystem) record(event string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, event)
}

// recorded returns a copy of the events seen so far.
func (s *syncOrderSystem) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.events...)
}

// newSyncOrderSystem builds the recording system over two monitors, so both
// the activation path and a move_monitor step can resolve a screen.
func newSyncOrderSystem() *syncOrderSystem {
	system := &syncOrderSystem{}
	screens := map[string]image.Rectangle{
		"A": image.Rect(0, 0, 1920, 1080),
		"B": image.Rect(0, 1080, 1920, 2160),
	}

	system.ScreenBoundsFunc = func(context.Context) (image.Rectangle, error) {
		system.record(eventScreenBounds)

		return screens["A"], nil
	}
	system.ScreenNamesFunc = func(context.Context) ([]string, error) {
		return []string{"A", "B"}, nil
	}
	system.ScreenBoundsByNameFunc = func(
		_ context.Context,
		name string,
	) (image.Rectangle, bool, error) {
		bounds, found := screens[name]

		return bounds, found, nil
	}

	return system
}

// syncOrderConfig enables both grid modes with enough layout for their
// managers to build.
func syncOrderConfig() *config.Config {
	return &config.Config{
		Grid: config.GridConfig{
			Enabled:    true,
			Characters: screenChangeGridCharacters,
			Hotkeys:    map[string]config.StringOrStringArray{},
		},
		RecursiveGrid: config.RecursiveGridConfig{
			Enabled:  true,
			Keys:     screenChangeGridCharacters,
			GridRows: 2,
			GridCols: 2,
			Hotkeys:  map[string]config.StringOrStringArray{},
		},
	}
}

// newSyncOrderHandler builds a handler with enough wiring for a full grid or
// recursive-grid activation to run end to end against the recording system.
func newSyncOrderHandler(system *syncOrderSystem) *Handler {
	handler := newHandlerWithState(handlerState{
		ctx:           context.Background(),
		logger:        zap.NewNop(),
		config:        syncOrderConfig(),
		appState:      state.NewAppState(),
		cursorState:   state.NewCursorState(),
		modifierState: state.NewModifierState(),
		system:        system,
		overlayPort:   &portmocks.MockOverlayPort{},
		actionService: services.NewActionService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockOverlayPort{},
			system,
			zap.NewNop(),
		),
		grid:   &components.GridComponent{Context: &gridcomponent.Context{}},
		scroll: &components.ScrollComponent{Context: &scroll.Context{}},
	})

	// The grid context stores the instance through a cell the component factory
	// wires; give the test context one the same way. The recursive-grid
	// component self-initializes on activation.
	gridInstance := new(*domainGrid.Grid)
	handler.grid.Context.SetGridInstance(gridInstance)

	return handler
}

// assertSyncBeforeScreenBounds fails unless the cursor sync happened, a
// ScreenBounds read happened, and every read came after the sync.
func assertSyncBeforeScreenBounds(t *testing.T, events []string) {
	t.Helper()

	syncIndex, boundsIndex := -1, -1

	for index, event := range events {
		if event == eventCursorSync && syncIndex == -1 {
			syncIndex = index
		}

		if event == eventScreenBounds && boundsIndex == -1 {
			boundsIndex = index
		}
	}

	if syncIndex == -1 {
		t.Fatalf("the cursor cache was never synced; events = %v", events)
	}

	if boundsIndex == -1 {
		t.Fatalf("ScreenBounds was never read; events = %v", events)
	}

	if boundsIndex < syncIndex {
		t.Fatalf(
			"ScreenBounds was read before the cursor sync, so monitor selection "+
				"used the stale cache; events = %v",
			events,
		)
	}
}

// TestActivateMode_Grid_SyncsCursorBeforeReadingScreenBounds pins the order the
// Wayland cache contract depends on: activation refreshes the platform's
// cursor cache before ScreenBounds decides which monitor the grid comes up on.
// Read the other way around, a user who physically moved the mouse since the
// last Neru-driven move gets the grid on the monitor Neru last put the cursor
// on, not the one they are looking at (#1279).
func TestActivateMode_Grid_SyncsCursorBeforeReadingScreenBounds(t *testing.T) {
	system := newSyncOrderSystem()
	handler := newSyncOrderHandler(system)

	handler.ActivateMode(modecmd.Activation{Mode: domain.ModeGrid})

	if got := handler.appState.CurrentMode(); got != domain.ModeGrid {
		t.Fatalf("mode after activation = %v, want grid", got)
	}

	assertSyncBeforeScreenBounds(t, system.recorded())
}

// TestActivateMode_RecursiveGrid_SyncsCursorBeforeReadingScreenBounds pins the
// same order for the other mode #1279 names: recursive-grid reads ScreenBounds
// on its own activation path, and must do so only after the cache refresh.
func TestActivateMode_RecursiveGrid_SyncsCursorBeforeReadingScreenBounds(t *testing.T) {
	system := newSyncOrderSystem()
	handler := newSyncOrderHandler(system)

	handler.ActivateMode(modecmd.Activation{Mode: domain.ModeRecursiveGrid})

	if got := handler.appState.CurrentMode(); got != domain.ModeRecursiveGrid {
		t.Fatalf("mode after activation = %v, want recursive-grid", got)
	}

	assertSyncBeforeScreenBounds(t, system.recorded())
}

// TestMoveMonitor_SyncsCursorBeforeResolvingCurrentMonitor pins the same order
// for the directional move: "next" is relative to the monitor under the
// cursor, and resolving it from the stale cache steps from wherever Neru last
// warped the cursor rather than from where the user's mouse is (#1279).
func TestMoveMonitor_SyncsCursorBeforeResolvingCurrentMonitor(t *testing.T) {
	system := newSyncOrderSystem()
	handler := newSyncOrderHandler(system)

	err := handler.MoveMonitor(context.Background(), MonitorDirectionNext)
	if err != nil {
		t.Fatalf("MoveMonitor() error = %v", err)
	}

	assertSyncBeforeScreenBounds(t, system.recorded())
}

// TestMoveMonitor_SyncRacesActivationAndReload is the -race guard on
// syncCursorPosition's dual lock context: MoveMonitor calls it under only
// moveMonitorMu, so it may run concurrently with an activation and a config
// reload, both of which mutate handler state under h.mu. It stays race-free
// only while the sync touches construction-time-only fields — an edit that
// makes it read h.config or a mode component fails here under -race.
func TestMoveMonitor_SyncRacesActivationAndReload(t *testing.T) {
	system := newSyncOrderSystem()
	handler := newSyncOrderHandler(system)

	const rounds = 100

	start := make(chan struct{})

	var waitGroup sync.WaitGroup

	waitGroup.Go(func() {
		<-start

		for range rounds {
			handler.ActivateMode(modecmd.Activation{Mode: domain.ModeGrid})
			handler.ExitMode()
		}
	})

	waitGroup.Go(func() {
		<-start

		for range rounds {
			handler.UpdateConfig(syncOrderConfig())
		}
	})

	waitGroup.Go(func() {
		<-start

		for range rounds {
			// The move's outcome is the other goroutines' business; what this
			// loop asserts is that the sync it runs unlocked races cleanly.
			_ = handler.MoveMonitor(context.Background(), MonitorDirectionNext)
		}
	})

	close(start)
	waitGroup.Wait()

	// The race detector is the point of this test; these assertions are the
	// deterministic floor under it. The activation goroutine's last
	// mode-changing call is an exit, and neither a monitor move nor a reload
	// changes the mode, so the handler must end idle — and both racing entry
	// points must actually have reached the sync for the race to have been
	// exercised at all.
	if got := handler.appState.CurrentMode(); got != domain.ModeIdle {
		t.Fatalf("mode after the last exit = %v, want idle", got)
	}

	syncs := 0

	for _, event := range system.recorded() {
		if event == eventCursorSync {
			syncs++
		}
	}

	if syncs < rounds {
		t.Fatalf(
			"saw %d cursor syncs across %d activations and %d monitor moves, want at least %d",
			syncs, rounds, rounds, rounds,
		)
	}
}
