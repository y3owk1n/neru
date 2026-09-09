package modes

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// newScaledGridHandler is a handler on a screen of the given size whose system
// reports the given display scale, the shape Windows and X11 have.
func newScaledGridHandler(cfg *config.Config, screen image.Rectangle, scale float64) *Handler {
	var gridInstance *domainGrid.Grid

	gridContext := &gridcomponent.Context{}
	gridContext.SetGridInstance(&gridInstance)

	return newHandlerWithState(handlerState{
		config: cfg,
		logger: zap.NewNop(),
		grid:   &components.GridComponent{Context: gridContext},
		system: &portmocks.MockSystemPort{
			ScreenBoundsFunc: func(_ context.Context) (image.Rectangle, error) {
				return screen, nil
			},
			ScreenScaleFunc: func(_ context.Context, _ image.Rectangle) (float64, error) {
				return scale, nil
			},
		},
		overlayPort:  &portmocks.MockOverlayPort{},
		screenBounds: screen,
	})
}

// TestCreateGridInstance_PlansTheGridInApparentUnits pins the mode-to-domain
// wiring for the display scale. A 4K monitor at 150% gets as many cells as a
// 2560x1440 monitor at 100%, each cell covering the same apparent size.
// Before this was wired, raising Windows display scaling made the cells
// smaller and denser.
func TestCreateGridInstance_PlansTheGridInApparentUnits(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Grid.Enabled = true
	cfg.ResolveGridLabels()

	scaled := newScaledGridHandler(cfg, image.Rect(0, 0, 3840, 2160), 1.5).createGridInstance()
	logical := newScaledGridHandler(cfg, image.Rect(0, 0, 2560, 1440), 1).createGridInstance()

	if got, want := len(scaled.Cells()), len(logical.Cells()); got != want {
		t.Fatalf("4K at 150%% planned %d cells, its 2560x1440 twin %d", got, want)
	}
}
