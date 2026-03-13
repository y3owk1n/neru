package modes

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	componentgrid "github.com/y3owk1n/neru/internal/app/components/grid"
	componenthints "github.com/y3owk1n/neru/internal/app/components/hints"
	componentrecursivegrid "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	domainrecursivegrid "github.com/y3owk1n/neru/internal/core/domain/recursivegrid"
	"github.com/y3owk1n/neru/internal/ui"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

type recursiveGridDrawCapture struct {
	overlay.NoOpManager

	bounds       image.Rectangle
	depth        int
	keys         string
	gridCols     int
	gridRows     int
	nextKeys     string
	nextGridCols int
	nextGridRows int
	calls        int
}

func (m *recursiveGridDrawCapture) DrawRecursiveGrid(
	bounds image.Rectangle,
	depth int,
	keys string,
	gridCols int,
	gridRows int,
	nextKeys string,
	nextGridCols int,
	nextGridRows int,
	_ componentrecursivegrid.Style,
) error {
	m.bounds = bounds
	m.depth = depth
	m.keys = keys
	m.gridCols = gridCols
	m.gridRows = gridRows
	m.nextKeys = nextKeys
	m.nextGridCols = nextGridCols
	m.nextGridRows = nextGridRows
	m.calls++

	return nil
}

func TestUpdateRecursiveGridOverlay_HidesTerminalSubKeyPreview(t *testing.T) {
	handler, capture := newRecursiveGridTestHandler(
		image.Rect(0, 0, 90, 90),
		nil,
		nil,
	)

	handler.updateRecursiveGridOverlay()

	assert.Equal(t, 1, capture.calls)
	assert.Equal(t, domainrecursivegrid.DefaultKeys, capture.keys)
	assert.Empty(t, capture.nextKeys)
	assert.Zero(t, capture.nextGridCols)
	assert.Zero(t, capture.nextGridRows)
}

func TestUpdateRecursiveGridOverlay_UsesNextDepthLayoutAndKeys(t *testing.T) {
	depthLayouts := map[int]domainrecursivegrid.DepthLayout{
		1: {
			GridCols: 3,
			GridRows: 3,
		},
	}
	depthKeys := map[int]string{
		1: "qweasdzxc",
	}

	handler, capture := newRecursiveGridTestHandler(
		image.Rect(0, 0, 300, 300),
		depthLayouts,
		depthKeys,
	)

	handler.updateRecursiveGridOverlay()

	assert.Equal(t, 1, capture.calls)
	assert.Equal(t, "qweasdzxc", capture.nextKeys)
	assert.Equal(t, 3, capture.nextGridCols)
	assert.Equal(t, 3, capture.nextGridRows)
}

func newRecursiveGridTestHandler(
	bounds image.Rectangle,
	depthLayouts map[int]domainrecursivegrid.DepthLayout,
	depthKeys map[int]string,
) (*Handler, *recursiveGridDrawCapture) {
	logger := zap.NewNop()
	capture := &recursiveGridDrawCapture{}
	renderer := ui.NewOverlayRenderer(
		capture,
		componenthints.StyleMode{},
		componentgrid.Style{},
		componentrecursivegrid.Style{},
	)

	manager := domainrecursivegrid.NewManagerWithLayers(
		bounds,
		domainrecursivegrid.DefaultKeys,
		"",
		"",
		nil,
		25,
		25,
		10,
		domainrecursivegrid.MinGridDimension,
		domainrecursivegrid.MinGridDimension,
		depthLayouts,
		depthKeys,
		nil,
		nil,
		logger,
	)

	handler := &Handler{
		logger:   logger,
		renderer: renderer,
		recursiveGrid: &components.RecursiveGridComponent{
			Manager: manager,
		},
	}

	return handler, capture
}
