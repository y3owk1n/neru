package modes

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/app/services"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// TestRefreshActiveModeForMonitorMove_ScansTheDisplayMovedTo covers hints with
// a capture strategy following move_monitor. The focused window stays on the
// display left behind, so a scan of it finds nothing on the display moved to,
// and hints closed instead of showing labels there.
func TestRefreshActiveModeForMonitorMove_ScansTheDisplayMovedTo(t *testing.T) {
	focusedWindow := image.Rect(100, 100, 900, 700)
	target := image.Rect(1920, 0, 3840, 1080)

	var scanned []image.Rectangle

	system := &portmocks.MockSystemPort{
		FocusedWindowBoundsFunc: func(context.Context) (image.Rectangle, bool, error) {
			return focusedWindow, true, nil
		},
		// The cursor has already been warped, so the screen under it is the
		// display moved to.
		ScreenBoundsFunc: func(context.Context) (image.Rectangle, error) {
			return target, nil
		},
	}

	vision := &portmocks.MockVisionPort{
		DetectContoursFunc: func(
			_ context.Context,
			bounds image.Rectangle,
			_ configpkg.HintsContourConfig,
		) ([]*element.Element, error) {
			scanned = append(scanned, bounds)

			found, err := element.NewElement(
				"contour-target",
				image.Rect(bounds.Min.X+40, bounds.Min.Y+40, bounds.Min.X+100, bounds.Min.Y+70),
				element.RoleButton,
			)
			if err != nil {
				return nil, err
			}

			return []*element.Element{found}, nil
		},
	}

	generator, err := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionNormal)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator() error = %v", err)
	}

	cfg := configpkg.DefaultConfig()

	handler := newHandlerWithState(handlerState{
		ctx:           context.Background(),
		logger:        zap.NewNop(),
		overlayPort:   &portmocks.MockOverlayPort{},
		system:        system,
		config:        cfg,
		appState:      state.NewAppState(),
		cursorState:   state.NewCursorState(),
		modifierState: state.NewModifierState(),
		hints:         &components.HintsComponent{Context: &hintscomponent.Context{}},
		scroll:        &components.ScrollComponent{Context: &scroll.Context{}},
		hintService: services.NewHintService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockOverlayPort{},
			system,
			generator,
			cfg.Hints,
			zap.NewNop(),
			vision,
		),
	})

	handler.appState.SetMode(domain.ModeHints)
	handler.hints.Context.SetStrategyOverride(domain.StrategyContour)

	handler.refreshActiveModeForMonitorMove(context.Background(), target)

	if len(scanned) != 1 || scanned[0] != target {
		t.Fatalf("the monitor move scanned %v, want the display moved to %v", scanned, target)
	}

	if got := handler.appState.CurrentMode(); got != domain.ModeHints {
		t.Errorf("mode after the monitor move = %v, want hints", got)
	}

	if got := handler.hints.Context.ActiveCaptureScope(); got != domain.CaptureScopeScreen {
		t.Errorf("active capture scope after the monitor move = %q, want %q",
			got, domain.CaptureScopeScreen)
	}
}
