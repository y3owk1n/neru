package ui_test

import (
	"errors"
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ui"
)

// errDraw is returned by the fake manager to check error propagation.
var errDraw = errors.New("draw failed")

// recursiveGridCall captures every argument DrawRecursiveGrid received. The
// signature has seven consecutive int/string parameters, so recording them
// individually is what makes a transposition detectable.
type recursiveGridCall struct {
	bounds         image.Rectangle
	depth          int
	keys           string
	gridCols       int
	gridRows       int
	nextKeys       string
	nextGridCols   int
	nextGridRows   int
	style          recursivegrid.Style
	virtualPointer recursivegrid.VirtualPointerState
}

// fakeManager records what the renderer forwarded. It embeds NoOpManager so
// that only the methods the renderer actually calls need overriding; if
// ManagerInterface grows a method, this keeps compiling.
type fakeManager struct {
	overlay.NoOpManager

	drawHintsErr error
	drawGridErr  error
	recursiveErr error

	hintsArg      []*hints.Hint
	hintsStyle    hints.StyleMode
	hintsCalls    int
	gridArg       *domainGrid.Grid
	gridInput     string
	gridStyle     grid.Style
	gridCalls     int
	subgridCell   *domainGrid.Cell
	subgridStyle  grid.Style
	matchesPrefix string
	hideUnmatched bool
	showCalls     int
	clearCalls    int
	resizeCalls   int
	indicatorX    int
	indicatorY    int
	recursive     recursiveGridCall
	recursiveHits int
}

func (m *fakeManager) DrawHintsWithStyle(hs []*hints.Hint, style hints.StyleMode) error {
	m.hintsArg = hs
	m.hintsStyle = style
	m.hintsCalls++

	return m.drawHintsErr
}

func (m *fakeManager) DrawGrid(g *domainGrid.Grid, input string, style grid.Style) error {
	m.gridArg = g
	m.gridInput = input
	m.gridStyle = style
	m.gridCalls++

	return m.drawGridErr
}

func (m *fakeManager) ShowSubgrid(cell *domainGrid.Cell, style grid.Style) {
	m.subgridCell = cell
	m.subgridStyle = style
}

func (m *fakeManager) UpdateGridMatches(prefix string) { m.matchesPrefix = prefix }
func (m *fakeManager) SetHideUnmatched(hide bool)      { m.hideUnmatched = hide }
func (m *fakeManager) Show()                           { m.showCalls++ }
func (m *fakeManager) Clear()                          { m.clearCalls++ }
func (m *fakeManager) ResizeToActiveScreen()           { m.resizeCalls++ }

func (m *fakeManager) DrawModeIndicator(x, y int) {
	m.indicatorX = x
	m.indicatorY = y
}

func (m *fakeManager) DrawRecursiveGrid(
	bounds image.Rectangle,
	depth int,
	keys string,
	gridCols int,
	gridRows int,
	nextKeys string,
	nextGridCols int,
	nextGridRows int,
	style recursivegrid.Style,
	virtualPointer recursivegrid.VirtualPointerState,
) error {
	m.recursive = recursiveGridCall{
		bounds:         bounds,
		depth:          depth,
		keys:           keys,
		gridCols:       gridCols,
		gridRows:       gridRows,
		nextKeys:       nextKeys,
		nextGridCols:   nextGridCols,
		nextGridRows:   nextGridRows,
		style:          style,
		virtualPointer: virtualPointer,
	}
	m.recursiveHits++

	return m.recursiveErr
}

// staticTheme is a config.ThemeProvider with a fixed appearance, so the styles
// built below depend only on the config values the test varies.
type staticTheme struct{ dark bool }

func (t staticTheme) IsDarkMode() bool { return t.dark }

// styleSet is a trio of styles built from one config, used as a unit so that
// "the renderer forwarded the right style" is checkable by equality.
type styleSet struct {
	hint      hints.StyleMode
	grid      grid.Style
	recursive recursivegrid.Style
}

// buildStyles derives a distinguishable style trio. Font size and border width
// are carried through by BuildStyle on every platform, so varying them yields
// styles that compare unequal wherever the test runs.
func buildStyles(fontSize, borderWidth int) styleSet {
	cfg := config.DefaultConfig()
	theme := staticTheme{}

	cfg.Hints.UI.FontSize = fontSize
	cfg.Hints.UI.BorderWidth = borderWidth
	cfg.Grid.UI.FontSize = fontSize
	cfg.Grid.UI.BorderWidth = borderWidth
	cfg.RecursiveGrid.UI.LineWidth = borderWidth

	return styleSet{
		hint:      hints.BuildStyle(cfg.Hints, theme),
		grid:      grid.BuildStyle(cfg.Grid, theme),
		recursive: recursivegrid.BuildStyle(cfg.RecursiveGrid, theme),
	}
}

// TestBuildStyles_ProducesDistinguishableStyles guards the premise of every
// style-forwarding assertion below: if two different configs happened to build
// equal styles, those assertions would pass vacuously.
func TestBuildStyles_ProducesDistinguishableStyles(t *testing.T) {
	first := buildStyles(12, 1)
	second := buildStyles(30, 6)

	if first.hint == second.hint {
		t.Error(
			"hint styles built from different configs compare equal; style assertions would be vacuous",
		)
	}

	if first.grid == second.grid {
		t.Error(
			"grid styles built from different configs compare equal; style assertions would be vacuous",
		)
	}

	if first.recursive == second.recursive {
		t.Error(
			"recursive-grid styles built from different configs compare equal; style assertions would be vacuous",
		)
	}
}

func newRenderer(t *testing.T, styles styleSet) (*ui.OverlayRenderer, *fakeManager) {
	t.Helper()

	manager := &fakeManager{}

	return ui.NewOverlayRenderer(manager, styles.hint, styles.grid, styles.recursive), manager
}

func TestOverlayRenderer_DrawHints_ForwardsHintsAndConfiguredStyle(t *testing.T) {
	styles := buildStyles(12, 1)
	renderer, manager := newRenderer(t, styles)

	drawn := []*hints.Hint{
		hints.NewHint("aa", image.Point{X: 1, Y: 2}, image.Point{X: 3, Y: 4}, ""),
	}

	err := renderer.DrawHints(drawn)
	if err != nil {
		t.Fatalf("DrawHints() error = %v, want nil", err)
	}

	if manager.hintsCalls != 1 {
		t.Fatalf("DrawHintsWithStyle called %d times, want 1", manager.hintsCalls)
	}

	if len(manager.hintsArg) != 1 || manager.hintsArg[0] != drawn[0] {
		t.Errorf(
			"DrawHintsWithStyle received %v, want the caller's slice %v",
			manager.hintsArg,
			drawn,
		)
	}

	if manager.hintsStyle != styles.hint {
		t.Error("DrawHintsWithStyle received a style other than the configured hint style")
	}
}

func TestOverlayRenderer_DrawHints_PropagatesManagerError(t *testing.T) {
	renderer, manager := newRenderer(t, buildStyles(12, 1))
	manager.drawHintsErr = errDraw

	err := renderer.DrawHints(nil)
	if !errors.Is(err, errDraw) {
		t.Errorf("DrawHints() error = %v, want %v", err, errDraw)
	}
}

func TestOverlayRenderer_DrawGrid_ForwardsGridInputAndConfiguredStyle(t *testing.T) {
	styles := buildStyles(12, 1)
	renderer, manager := newRenderer(t, styles)

	testGrid := domainGrid.NewGrid("abc", image.Rect(0, 0, 100, 100), zap.NewNop())

	err := renderer.DrawGrid(testGrid, "ab")
	if err != nil {
		t.Fatalf("DrawGrid() error = %v, want nil", err)
	}

	if manager.gridArg != testGrid {
		t.Errorf("DrawGrid received grid %p, want the caller's grid %p", manager.gridArg, testGrid)
	}

	if manager.gridInput != "ab" {
		t.Errorf("DrawGrid received input %q, want %q", manager.gridInput, "ab")
	}

	if manager.gridStyle != styles.grid {
		t.Error("DrawGrid received a style other than the configured grid style")
	}
}

func TestOverlayRenderer_DrawGrid_PropagatesManagerError(t *testing.T) {
	renderer, manager := newRenderer(t, buildStyles(12, 1))
	manager.drawGridErr = errDraw

	err := renderer.DrawGrid(nil, "")
	if !errors.Is(err, errDraw) {
		t.Errorf("DrawGrid() error = %v, want %v", err, errDraw)
	}
}

// TestOverlayRenderer_ShowSubgrid_UsesGridStyleNotRecursiveStyle pins which of
// the three stored styles the subgrid path uses. Both grid and recursive-grid
// styles are plausible choices at this call site, so an incorrect swap would
// otherwise render subgrids with the wrong appearance silently.
func TestOverlayRenderer_ShowSubgrid_UsesGridStyleNotRecursiveStyle(t *testing.T) {
	styles := buildStyles(12, 1)
	renderer, manager := newRenderer(t, styles)

	testGrid := domainGrid.NewGrid("abc", image.Rect(0, 0, 100, 100), zap.NewNop())

	cells := testGrid.Cells()
	if len(cells) == 0 {
		t.Fatal("test grid produced no cells")
	}

	renderer.ShowSubgrid(cells[0])

	if manager.subgridCell != cells[0] {
		t.Errorf("ShowSubgrid received cell %p, want %p", manager.subgridCell, cells[0])
	}

	if manager.subgridStyle != styles.grid {
		t.Error("ShowSubgrid received a style other than the configured grid style")
	}
}

// TestOverlayRenderer_DrawRecursiveGrid_ForwardsEveryArgumentInOrder is the
// highest-value case in this file: the manager call takes seven consecutive
// int/string parameters describing the current and next depth's layout, which
// makes an accidental transposition both easy to write and invisible to the
// compiler. Every value here is distinct so any swap changes the recorded call.
func TestOverlayRenderer_DrawRecursiveGrid_ForwardsEveryArgumentInOrder(t *testing.T) {
	styles := buildStyles(12, 1)
	renderer, manager := newRenderer(t, styles)

	want := recursiveGridCall{
		bounds:       image.Rect(10, 20, 310, 220),
		depth:        3,
		keys:         "qwer",
		gridCols:     4,
		gridRows:     5,
		nextKeys:     "asdf",
		nextGridCols: 6,
		nextGridRows: 7,
		style:        styles.recursive,
		virtualPointer: recursivegrid.VirtualPointerState{
			Visible:   true,
			Position:  image.Point{X: 42, Y: 43},
			Size:      8,
			FillColor: "#ff0000",
			Char:      "x",
			FontName:  "Menlo",
		},
	}

	err := renderer.DrawRecursiveGrid(
		want.bounds,
		want.depth,
		want.keys,
		want.gridCols,
		want.gridRows,
		want.nextKeys,
		want.nextGridCols,
		want.nextGridRows,
		want.virtualPointer,
	)
	if err != nil {
		t.Fatalf("DrawRecursiveGrid() error = %v, want nil", err)
	}

	if manager.recursiveHits != 1 {
		t.Fatalf("DrawRecursiveGrid called %d times, want 1", manager.recursiveHits)
	}

	got := manager.recursive

	if got.bounds != want.bounds {
		t.Errorf("bounds = %v, want %v", got.bounds, want.bounds)
	}

	if got.depth != want.depth {
		t.Errorf("depth = %d, want %d", got.depth, want.depth)
	}

	if got.keys != want.keys {
		t.Errorf("keys = %q, want %q", got.keys, want.keys)
	}

	if got.gridCols != want.gridCols {
		t.Errorf("gridCols = %d, want %d", got.gridCols, want.gridCols)
	}

	if got.gridRows != want.gridRows {
		t.Errorf("gridRows = %d, want %d", got.gridRows, want.gridRows)
	}

	if got.nextKeys != want.nextKeys {
		t.Errorf("nextKeys = %q, want %q", got.nextKeys, want.nextKeys)
	}

	if got.nextGridCols != want.nextGridCols {
		t.Errorf("nextGridCols = %d, want %d", got.nextGridCols, want.nextGridCols)
	}

	if got.nextGridRows != want.nextGridRows {
		t.Errorf("nextGridRows = %d, want %d", got.nextGridRows, want.nextGridRows)
	}

	if got.virtualPointer != want.virtualPointer {
		t.Errorf("virtualPointer = %+v, want %+v", got.virtualPointer, want.virtualPointer)
	}

	if got.style != styles.recursive {
		t.Error("DrawRecursiveGrid received a style other than the configured recursive-grid style")
	}
}

func TestOverlayRenderer_DrawRecursiveGrid_PropagatesManagerError(t *testing.T) {
	renderer, manager := newRenderer(t, buildStyles(12, 1))
	manager.recursiveErr = errDraw

	err := renderer.DrawRecursiveGrid(
		image.Rectangle{}, 0, "", 0, 0, "", 0, 0, recursivegrid.VirtualPointerState{},
	)
	if !errors.Is(err, errDraw) {
		t.Errorf("DrawRecursiveGrid() error = %v, want %v", err, errDraw)
	}
}

// TestOverlayRenderer_UpdateConfig_SwapsAllThreeStyles checks that a config
// reload actually reaches subsequent draws. The renderer caches the styles by
// value, so a missed assignment in UpdateConfig would leave one overlay
// rendering with the pre-reload appearance indefinitely.
func TestOverlayRenderer_UpdateConfig_SwapsAllThreeStyles(t *testing.T) {
	before := buildStyles(12, 1)
	after := buildStyles(30, 6)

	renderer, manager := newRenderer(t, before)

	renderer.UpdateConfig(after.hint, after.grid, after.recursive)

	err := renderer.DrawHints(nil)
	if err != nil {
		t.Fatalf("DrawHints() error = %v", err)
	}

	if manager.hintsStyle != after.hint {
		t.Error("DrawHints still used the pre-UpdateConfig hint style")
	}

	err = renderer.DrawGrid(nil, "")
	if err != nil {
		t.Fatalf("DrawGrid() error = %v", err)
	}

	if manager.gridStyle != after.grid {
		t.Error("DrawGrid still used the pre-UpdateConfig grid style")
	}

	renderer.ShowSubgrid(nil)

	if manager.subgridStyle != after.grid {
		t.Error("ShowSubgrid still used the pre-UpdateConfig grid style")
	}

	err = renderer.DrawRecursiveGrid(
		image.Rectangle{}, 0, "", 0, 0, "", 0, 0, recursivegrid.VirtualPointerState{},
	)
	if err != nil {
		t.Fatalf("DrawRecursiveGrid() error = %v", err)
	}

	if manager.recursive.style != after.recursive {
		t.Error("DrawRecursiveGrid still used the pre-UpdateConfig recursive-grid style")
	}
}

// TestOverlayRenderer_PassThroughMethods covers the methods that carry no style
// but must still reach the manager with their arguments intact.
func TestOverlayRenderer_PassThroughMethods(t *testing.T) {
	renderer, manager := newRenderer(t, buildStyles(12, 1))

	renderer.UpdateGridMatches("abc")

	if manager.matchesPrefix != "abc" {
		t.Errorf("UpdateGridMatches forwarded %q, want %q", manager.matchesPrefix, "abc")
	}

	renderer.SetHideUnmatched(true)

	if !manager.hideUnmatched {
		t.Error("SetHideUnmatched(true) did not reach the manager")
	}

	renderer.SetHideUnmatched(false)

	if manager.hideUnmatched {
		t.Error("SetHideUnmatched(false) did not reach the manager")
	}

	renderer.Show()
	renderer.Clear()
	renderer.ResizeActive()

	if manager.showCalls != 1 {
		t.Errorf("Show() reached the manager %d times, want 1", manager.showCalls)
	}

	if manager.clearCalls != 1 {
		t.Errorf("Clear() reached the manager %d times, want 1", manager.clearCalls)
	}

	if manager.resizeCalls != 1 {
		t.Errorf("ResizeActive() reached the manager %d times, want 1", manager.resizeCalls)
	}

	renderer.DrawModeIndicator(11, 22)

	if manager.indicatorX != 11 || manager.indicatorY != 22 {
		t.Errorf(
			"DrawModeIndicator forwarded (%d, %d), want (11, 22)",
			manager.indicatorX, manager.indicatorY,
		)
	}
}
