package modes

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/services/modeindicator"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// newIndicatorPollingHandler builds a handler whose polling tick draws the
// mode indicator: hints mode active, its indicator enabled, cursor readable.
func newIndicatorPollingHandler(
	systemMock *portmocks.MockSystemPort,
	overlayPort *portmocks.MockOverlayPort,
) *Handler {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	return newHandlerWithState(handlerState{
		logger: zap.NewNop(),
		config: &configpkg.Config{
			ModeIndicator: configpkg.ModeIndicatorConfig{
				Hints: configpkg.ModeIndicatorModeConfig{Enabled: true},
			},
		},
		appState:             appState,
		system:               systemMock,
		overlayPort:          overlayPort,
		modeIndicatorService: modeindicator.NewService(systemMock, overlayPort),
		screenBounds:         image.Rect(0, 0, 100, 100),
	})
}

func TestPollIndicatorsOnce_DrawsOnlyAfterReleasingLock(t *testing.T) {
	var handler *Handler

	var drewModeIndicator, flushed bool

	// Single-goroutine test: nothing else contends for h.mu, so a failed
	// TryLock inside a draw means the polling path still holds the lock.
	assertLockFree := func(site string) {
		t.Helper()

		if handler.mu.TryLock() {
			handler.mu.Unlock()

			return
		}

		t.Errorf("%s called while the handler lock was held", site)
	}

	systemMock := &portmocks.MockSystemPort{
		CursorPositionFunc: func(context.Context) (image.Point, error) {
			return image.Pt(10, 10), nil
		},
	}

	var showedModeIndicator bool

	overlayPort := &portmocks.MockOverlayPort{
		DrawModeIndicatorFunc: func(_, _ int) {
			drewModeIndicator = true

			assertLockFree("DrawModeIndicator")
		},
		// Visibility reaches the platform the same way a draw does — on Linux
		// hiding an indicator erases the rectangle it painted — so the tick
		// must have released the lock before it asks for either.
		ShowIndicatorFunc: func(indicator ports.Indicator) {
			if indicator == ports.ModeIndicator {
				showedModeIndicator = true
			}

			assertLockFree("ShowIndicator")
		},
		HideIndicatorFunc: func(_ ports.Indicator) {
			assertLockFree("HideIndicator")
		},
		FlushFunc: func() {
			flushed = true

			assertLockFree("Flush")
		},
	}

	handler = newIndicatorPollingHandler(systemMock, overlayPort)

	handler.pollIndicatorsOnce(context.Background())

	if !drewModeIndicator {
		t.Error("expected the tick to draw the mode indicator")
	}

	if !showedModeIndicator {
		t.Error("expected the tick to show the mode indicator")
	}

	if !flushed {
		t.Error("expected the tick to flush the overlay")
	}
}

func TestPollIndicatorsOnce_AdoptsChangedScreenBoundsWithoutDrawing(t *testing.T) {
	newBounds := image.Rect(0, 0, 300, 300)

	systemMock := &portmocks.MockSystemPort{
		CursorPositionFunc: func(context.Context) (image.Point, error) {
			// Outside the handler's recorded bounds, triggering the re-read.
			return image.Pt(200, 200), nil
		},
		ScreenBoundsFunc: func(context.Context) (image.Rectangle, error) {
			return newBounds, nil
		},
	}

	var drew, flushed bool

	overlayPort := &portmocks.MockOverlayPort{
		DrawModeIndicatorFunc: func(_, _ int) { drew = true },
		FlushFunc:             func() { flushed = true },
	}

	handler := newIndicatorPollingHandler(systemMock, overlayPort)

	handler.pollIndicatorsOnce(context.Background())

	if handler.screenBounds != newBounds {
		t.Fatalf("screenBounds = %v, want %v", handler.screenBounds, newBounds)
	}

	if drew || flushed {
		t.Error("a resize tick must skip the draw so the next tick uses the new bounds")
	}
}

func TestPollIndicatorsOnce_SkipsTickWhenLockContended(t *testing.T) {
	systemMock := &portmocks.MockSystemPort{
		CursorPositionFunc: func(context.Context) (image.Point, error) {
			return image.Pt(10, 10), nil
		},
	}

	var drew, flushed bool

	overlayPort := &portmocks.MockOverlayPort{
		DrawModeIndicatorFunc: func(_, _ int) { drew = true },
		FlushFunc:             func() { flushed = true },
	}

	handler := newIndicatorPollingHandler(systemMock, overlayPort)

	handler.mu.Lock()
	handler.pollIndicatorsOnce(context.Background())
	handler.mu.Unlock()

	if drew || flushed {
		t.Error("a contended tick must be skipped entirely")
	}
}

func TestStickyIndicatorAnchor_UsesGridSelectionWhenCursorFollowDisabled(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeGrid)

	handler := newHandlerWithState(handlerState{
		appState: appState,
		logger:   zap.NewNop(),
		grid: &components.GridComponent{
			Context: &gridcomponent.Context{},
		},
	})

	handler.grid.Context.SetCursorFollowSelection(false)
	handler.grid.Context.SetSelectionPoint(image.Pt(40, 60))

	got := handler.stickyIndicatorAnchor(image.Pt(10, 20))

	want := image.Pt(40, 60)
	if got != want {
		t.Fatalf("stickyIndicatorAnchor() = %v, want %v", got, want)
	}
}

func TestStickyIndicatorAnchor_UsesRecursiveGridSelectionWhenCursorFollowDisabled(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	handler := newHandlerWithState(handlerState{
		appState: appState,
		logger:   zap.NewNop(),
		recursiveGrid: &components.RecursiveGridComponent{
			Context: &recursivegridcomponent.Context{},
		},
	})

	handler.recursiveGrid.Context.SetCursorFollowSelection(false)
	handler.recursiveGrid.Context.SetSelectionPoint(image.Pt(75, 25))

	got := handler.stickyIndicatorAnchor(image.Pt(10, 20))

	want := image.Pt(75, 25)
	if got != want {
		t.Fatalf("stickyIndicatorAnchor() = %v, want %v", got, want)
	}
}

func TestStickyIndicatorAnchor_UsesCursorWhenGridFollowsSelection(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeGrid)

	handler := newHandlerWithState(handlerState{
		appState: appState,
		logger:   zap.NewNop(),
		grid: &components.GridComponent{
			Context: &gridcomponent.Context{},
		},
	})

	handler.grid.Context.SetCursorFollowSelection(true)
	handler.grid.Context.SetSelectionPoint(image.Pt(40, 60))

	got := handler.stickyIndicatorAnchor(image.Pt(10, 20))

	want := image.Pt(10, 20)
	if got != want {
		t.Fatalf("stickyIndicatorAnchor() = %v, want %v", got, want)
	}
}
