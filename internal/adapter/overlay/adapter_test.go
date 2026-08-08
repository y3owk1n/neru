package overlay_test

import (
	"context"
	"image"
	"slices"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	rendergrid "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	renderhints "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	renderrecursivegrid "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/ports"
)

// testStyles is a StyleOwner for the tests that never assert on appearance:
// every Style it hands out is the zero one, and re-resolving is a no-op.
type testStyles struct{}

func (testStyles) Style() overlay.Style   { return overlay.Style{} }
func (testStyles) Apply(_ *config.Config) {}
func (testStyles) Refresh()               {}

type supportedManager struct {
	headlessManager
}

type stubManager struct {
	headlessManager
}

func (m *supportedManager) OverlayCapabilities() ports.FeatureCapability {
	return ports.FeatureCapability{
		Status: ports.FeatureStatusSupported,
		Detail: "test overlay available",
	}
}

func (m *stubManager) OverlayCapabilities() ports.FeatureCapability {
	return ports.FeatureCapability{
		Status: ports.FeatureStatusStub,
		Detail: "test overlay unavailable",
	}
}

func TestAdapterHealth_ReturnsNilForHeadlessOverlayManager(t *testing.T) {
	adapter := overlay.NewAdapter(
		&headlessManager{},
		testStyles{},
		zap.NewNop(),
	)

	err := adapter.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error = %v, want nil", err)
	}
}

func TestAdapterHealth_ReturnsNilForSupportedOverlayManager(t *testing.T) {
	adapter := overlay.NewAdapter(
		&supportedManager{},
		testStyles{},
		zap.NewNop(),
	)

	err := adapter.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error = %v, want nil", err)
	}
}

func TestAdapterHealth_ReturnsNotSupportedForStubOverlayManager(t *testing.T) {
	adapter := overlay.NewAdapter(
		&stubManager{},
		testStyles{},
		zap.NewNop(),
	)

	err := adapter.Health(context.Background())
	if err == nil {
		t.Fatal("Health() error = nil, want not supported error")
	}
}

// screenManager records the state a user could observe: whether the overlay is
// on screen, which mode it is in, and what was last drawn on it. It deliberately
// records no call sequence — which calls the adapter uses to realize a Frame is
// exactly what the Frame port exists to be free to change.
type screenManager struct {
	headlessManager

	visible bool
	mode    overlay.Mode
	resizes int
	drawn   []*renderhints.Hint
	cleared int

	searchQuery   string
	searchResults int
	searchFrame   renderhints.SearchInputFrame
	searchDraws   int
	searchHides   int

	grid              *domainGrid.Grid
	gridInput         string
	gridDraws         int
	clearedBeforeGrid int
	matchPrefix       string
	hideUnmatched     bool
	subgridCell       *domainGrid.Cell

	recursiveGrid      recursiveGridDraw
	recursiveGridDraws int

	pointer gridPointerDraw
}

// recursiveGridDraw is one recursive-grid drawn on the overlay, kept as the
// values a caller handed over rather than as the call that carried them.
type recursiveGridDraw struct {
	bounds   image.Rectangle
	depth    int
	keys     string
	dims     domain.GridDimensions
	nextKeys string
	nextDims domain.GridDimensions
	pointer  renderrecursivegrid.VirtualPointerState
}

// gridPointerDraw is the pointer stand-in a grid surface was last asked for.
type gridPointerDraw struct {
	mode      overlay.Mode
	visible   bool
	point     image.Point
	size      int
	fillColor string
}

func newScreenManager() *screenManager {
	return &screenManager{mode: overlay.ModeIdle}
}

func (m *screenManager) Show() { m.visible = true }

func (m *screenManager) Hide() { m.visible = false }

func (m *screenManager) Clear() { m.cleared++ }

func (m *screenManager) ResizeToActiveScreen() { m.resizes++ }

func (m *screenManager) SwitchTo(next overlay.Mode) { m.mode = next }

func (m *screenManager) Mode() overlay.Mode { return m.mode }

func (m *screenManager) DrawHintsWithStyle(
	drawn []*renderhints.Hint,
	_ renderhints.StyleMode,
) error {
	m.drawn = drawn

	return nil
}

func (m *screenManager) DrawHintSearchInput(
	query string,
	resultCount int,
	frame renderhints.SearchInputFrame,
	_ renderhints.SearchInputStyle,
) error {
	m.searchQuery = query
	m.searchResults = resultCount
	m.searchFrame = frame
	m.searchDraws++

	return nil
}

func (m *screenManager) HideHintSearchInput() { m.searchHides++ }

func (m *screenManager) DrawGrid(
	drawn *domainGrid.Grid,
	input string,
	_ rendergrid.Style,
) error {
	m.grid = drawn
	m.gridInput = input
	m.gridDraws++
	m.clearedBeforeGrid = m.cleared

	return nil
}

func (m *screenManager) UpdateGridMatches(prefix string) {
	m.matchPrefix = prefix
}

func (m *screenManager) SetHideUnmatched(hide bool) { m.hideUnmatched = hide }

func (m *screenManager) ShowSubgrid(cell *domainGrid.Cell, _ rendergrid.Style) {
	m.subgridCell = cell
}

func (m *screenManager) DrawRecursiveGrid(
	bounds image.Rectangle,
	depth int,
	keys string,
	dims domain.GridDimensions,
	nextKeys string,
	nextDims domain.GridDimensions,
	_ renderrecursivegrid.Style,
	virtualPointer renderrecursivegrid.VirtualPointerState,
) error {
	m.recursiveGrid = recursiveGridDraw{
		bounds:   bounds,
		depth:    depth,
		keys:     keys,
		dims:     dims,
		nextKeys: nextKeys,
		nextDims: nextDims,
		pointer:  virtualPointer,
	}
	m.recursiveGridDraws++

	return nil
}

func (m *screenManager) DrawGridPointer(
	mode overlay.Mode,
	point image.Point,
	size int,
	fillColor string,
) {
	m.pointer = gridPointerDraw{
		mode:      mode,
		visible:   true,
		point:     point,
		size:      size,
		fillColor: fillColor,
	}
}

func (m *screenManager) HideGridPointer(mode overlay.Mode) {
	m.pointer = gridPointerDraw{mode: mode}
}

func (m *screenManager) drawnLabels() []string {
	labels := make([]string, len(m.drawn))
	for index, drawn := range m.drawn {
		labels[index] = drawn.Label()
	}

	return labels
}

// hintsFrame builds a one-label hints Frame on the screen given, with the
// label positioned at the point given in global coordinates.
func hintsFrame(
	t *testing.T,
	screen image.Rectangle,
	label string,
	position image.Point,
) ports.HintsFrame {
	t.Helper()

	target, elementErr := element.NewElement(
		element.ID(label),
		image.Rectangle{Min: position, Max: position.Add(image.Pt(10, 10))},
		element.Role("button"),
	)
	if elementErr != nil {
		t.Fatalf("NewElement() error = %v", elementErr)
	}

	drawn, hintErr := hint.NewHint(label, target, position)
	if hintErr != nil {
		t.Fatalf("NewHint() error = %v", hintErr)
	}

	return ports.HintsFrame{Screen: screen, Hints: []*hint.Interface{drawn}}
}

// TestAdapterShowFrame_PutsTheFrameOnScreenInItsMode is the acceptance the
// Frame port exists for: a caller hands over what should be on screen, and
// what ends up there is visible, in the frame's mode, showing the frame's
// content. Nothing about the order the adapter got there in is asserted.
func TestAdapterShowFrame_PutsTheFrameOnScreenInItsMode(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	frame := hintsFrame(t, image.Rect(0, 0, 1920, 1080), "ab", image.Pt(100, 200))

	err := adapter.ShowFrame(context.Background(), frame)
	if err != nil {
		t.Fatalf("ShowFrame() error = %v", err)
	}

	if !manager.visible {
		t.Error("the overlay is not on screen after a frame was shown")
	}

	if manager.mode != overlay.ModeHints {
		t.Errorf("overlay mode = %q, want %q", manager.mode, overlay.ModeHints)
	}

	if got := manager.drawnLabels(); !slices.Equal(got, []string{"ab"}) {
		t.Errorf("labels drawn = %v, want [ab]", got)
	}

	if !adapter.IsVisible() {
		t.Error("IsVisible() = false after a frame was shown")
	}
}

// TestAdapterShowFrame_DrawsHintsInScreenLocalCoordinates pins the one
// conversion a Frame deliberately does not carry: hints arrive in global
// coordinates and the overlay draws in the active screen's own.
func TestAdapterShowFrame_DrawsHintsInScreenLocalCoordinates(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	frame := hintsFrame(t, image.Rect(1920, 0, 3840, 1080), "cd", image.Pt(2020, 300))

	err := adapter.ShowFrame(context.Background(), frame)
	if err != nil {
		t.Fatalf("ShowFrame() error = %v", err)
	}

	if len(manager.drawn) != 1 {
		t.Fatalf("hints drawn = %d, want 1", len(manager.drawn))
	}

	want := image.Pt(100, 300)
	if got := manager.drawn[0].Position(); got != want {
		t.Errorf("hint drawn at %v, want %v: the screen origin was not subtracted", got, want)
	}
}

// TestAdapterRedrawFrame_DrawsWithoutTheWindowSequence pins the decision that
// keeps grid narrowing and hint narrowing as fast as they are: a redraw of a
// surface already up costs a draw and nothing else (ADR 0003).
func TestAdapterRedrawFrame_DrawsWithoutTheWindowSequence(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	screen := image.Rect(0, 0, 1920, 1080)

	showErr := adapter.ShowFrame(
		context.Background(),
		hintsFrame(t, screen, "ab", image.Pt(10, 10)),
	)
	if showErr != nil {
		t.Fatalf("ShowFrame() error = %v", showErr)
	}

	resizesAfterShow := manager.resizes

	redrawErr := adapter.RedrawFrame(
		context.Background(),
		hintsFrame(t, screen, "ad", image.Pt(10, 10)),
	)
	if redrawErr != nil {
		t.Fatalf("RedrawFrame() error = %v", redrawErr)
	}

	if manager.resizes != resizesAfterShow {
		t.Errorf(
			"resizes after a redraw = %d, want %d: a redraw is paying for the window sequence",
			manager.resizes,
			resizesAfterShow,
		)
	}

	if got := manager.drawnLabels(); !slices.Equal(got, []string{"ad"}) {
		t.Errorf("labels drawn = %v, want [ad]: the redraw did not reach the screen", got)
	}
}

// gridFrame builds a grid Frame over a small labeled grid.
func gridFrame(t *testing.T, input string) ports.GridFrame {
	t.Helper()

	drawn := domainGrid.NewGridWithLabels(
		"abcd",
		"",
		"",
		image.Rect(0, 0, 400, 300),
		zap.NewNop(),
	)
	if drawn == nil {
		t.Fatal("NewGridWithLabels() returned nil")
	}

	return ports.GridFrame{Grid: drawn, Input: input}
}

// TestAdapterShowFrame_PutsTheGridOnScreenInGridMode is the grid half of the
// acceptance the Frame port exists for: a mode says a grid should be on
// screen, and the adapter owns everything it takes to get it there.
func TestAdapterShowFrame_PutsTheGridOnScreenInGridMode(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	frame := gridFrame(t, "a")

	err := adapter.ShowFrame(context.Background(), frame)
	if err != nil {
		t.Fatalf("ShowFrame() error = %v", err)
	}

	if !manager.visible {
		t.Error("the overlay is not on screen after a grid frame was shown")
	}

	if manager.mode != overlay.ModeGrid {
		t.Errorf("overlay mode = %q, want %q", manager.mode, overlay.ModeGrid)
	}

	if manager.grid != frame.Grid {
		t.Error("the grid the frame carried never reached the screen")
	}

	if manager.gridInput != "a" {
		t.Errorf("grid drawn narrowed to %q, want %q", manager.gridInput, "a")
	}
}

// TestAdapterShowFrame_DrawsAGridOnACleanSurface pins the one thing a grid
// transition needs that a hint one does not: the surface may be holding a
// subgrid this grid replaces, and the backend's incremental path cannot diff
// that away.
func TestAdapterShowFrame_DrawsAGridOnACleanSurface(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	err := adapter.ShowFrame(context.Background(), gridFrame(t, ""))
	if err != nil {
		t.Fatalf("ShowFrame() error = %v", err)
	}

	if manager.gridDraws != 1 {
		t.Fatalf("grid draws = %d, want 1", manager.gridDraws)
	}

	if manager.clearedBeforeGrid == 0 {
		t.Error("the grid was drawn onto a surface that was never cleared")
	}
}

// TestAdapterRedrawFrame_DrawsAGridWithoutClearingIt is the other half: a
// redraw leaves the surface alone, so the indicators painted on it survive a
// theme change instead of blinking out until the next poll.
func TestAdapterRedrawFrame_DrawsAGridWithoutClearingIt(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	err := adapter.RedrawFrame(context.Background(), gridFrame(t, "a"))
	if err != nil {
		t.Fatalf("RedrawFrame() error = %v", err)
	}

	if manager.gridDraws != 1 {
		t.Fatalf("grid draws = %d, want 1", manager.gridDraws)
	}

	if manager.cleared != 0 {
		t.Errorf("clears during a grid redraw = %d, want 0", manager.cleared)
	}
}

// TestAdapterRedrawFrame_ErasesASubgridTheGridReplaces is what a user sees on
// the keystroke that leaves a subgrid: the main grid comes back and the
// subgrid does not stay drawn over it. The overlay knows a subgrid is up
// because it drew it, so no caller has to tell it.
func TestAdapterRedrawFrame_ErasesASubgridTheGridReplaces(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	cells := gridFrame(t, "").Grid.Cells()
	if len(cells) == 0 {
		t.Fatal("fixture grid has no cells")
	}

	adapter.ShowGridSubgrid(cells[0])

	err := adapter.RedrawFrame(context.Background(), gridFrame(t, ""))
	if err != nil {
		t.Fatalf("RedrawFrame() error = %v", err)
	}

	if manager.clearedBeforeGrid == 0 {
		t.Error("the subgrid was left on the surface the grid was redrawn onto")
	}

	// And only that once: the subgrid is gone, so the next redraw is a plain
	// one again.
	clearedAfterRestore := manager.cleared

	redrawErr := adapter.RedrawFrame(context.Background(), gridFrame(t, "a"))
	if redrawErr != nil {
		t.Fatalf("RedrawFrame() error = %v", redrawErr)
	}

	if manager.cleared != clearedAfterRestore {
		t.Errorf("clears = %d after the second redraw, want %d: the subgrid was erased twice",
			manager.cleared, clearedAfterRestore)
	}
}

// TestAdapterShowFrame_PutsTheRecursiveGridOnScreen pins that every value the
// recursive-grid draw needs travels on the frame, including the pointer that
// rides the same surface.
func TestAdapterShowFrame_PutsTheRecursiveGridOnScreen(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	// The two layouts are deliberately non-square, and transposes of each
	// other: a draw that swapped a layout's rows for its columns on the way
	// through would still divide the region into the same number of cells, and
	// only a shape this asymmetric fails on it.
	frame := ports.RecursiveGridFrame{
		Bounds: image.Rect(10, 20, 210, 220),
		Depth:  2,
		Layout: ports.RecursiveGridLayout{
			Keys:       "qwerty",
			Dimensions: domain.GridDimensions{Rows: 2, Cols: 3},
		},
		NextLayout: ports.RecursiveGridLayout{
			Keys:       "asdfgh",
			Dimensions: domain.GridDimensions{Rows: 3, Cols: 2},
		},
		Pointer: ports.GridPointer{Visible: true, Position: image.Pt(30, 40)},
	}

	err := adapter.ShowFrame(context.Background(), frame)
	if err != nil {
		t.Fatalf("ShowFrame() error = %v", err)
	}

	if manager.mode != overlay.ModeRecursiveGrid {
		t.Errorf("overlay mode = %q, want %q", manager.mode, overlay.ModeRecursiveGrid)
	}

	drawn := manager.recursiveGrid
	if drawn.bounds != frame.Bounds || drawn.depth != frame.Depth ||
		drawn.keys != frame.Layout.Keys {
		t.Errorf("recursive grid drawn as %+v, want the frame's bounds, depth and keys", drawn)
	}

	if drawn.dims != frame.Layout.Dimensions {
		t.Errorf("recursive grid drawn as %+v, want %+v",
			drawn.dims, frame.Layout.Dimensions)
	}

	if drawn.nextKeys != frame.NextLayout.Keys || drawn.nextDims != frame.NextLayout.Dimensions {
		t.Error("the next depth's preview never reached the screen")
	}

	if !drawn.pointer.Visible || drawn.pointer.Position != frame.Pointer.Position {
		t.Errorf("pointer drawn as %+v, want visible at %v",
			drawn.pointer, frame.Pointer.Position)
	}
}

// TestAdapterRedrawFrame_DrawsTheRecursiveGridWithoutClearingIt pins the
// difference from grid: the recursive-grid draw repaints its whole surface, so
// a redraw that cleared first would throw away the animation state a user sees
// as the grid zooming in.
func TestAdapterRedrawFrame_DrawsTheRecursiveGridWithoutClearingIt(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	frame := ports.RecursiveGridFrame{
		Bounds: image.Rect(0, 0, 100, 100),
		Layout: ports.RecursiveGridLayout{
			Keys:       "qwer",
			Dimensions: domain.GridDimensions{Rows: 2, Cols: 2},
		},
	}

	showErr := adapter.ShowFrame(context.Background(), frame)
	if showErr != nil {
		t.Fatalf("ShowFrame() error = %v", showErr)
	}

	clearedAfterShow := manager.cleared

	redrawErr := adapter.RedrawFrame(context.Background(), frame)
	if redrawErr != nil {
		t.Fatalf("RedrawFrame() error = %v", redrawErr)
	}

	if manager.cleared != clearedAfterShow {
		t.Errorf("clears after a recursive-grid redraw = %d, want %d",
			manager.cleared, clearedAfterShow)
	}

	if manager.recursiveGridDraws != 2 {
		t.Errorf("recursive grid draws = %d, want 2", manager.recursiveGridDraws)
	}
}

// TestAdapterGridUpdates_ReachTheOverlayWithoutAFrame is ADR 0003's hot half:
// the values that change on every keystroke in grid mode travel as plain
// calls, and none of them costs a frame or the window sequence.
func TestAdapterGridUpdates_ReachTheOverlayWithoutAFrame(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	adapter.SetGridHideUnmatched(true)
	adapter.UpdateGridMatches("ab")

	if !manager.hideUnmatched {
		t.Error("unmatched cells were not asked to hide")
	}

	if manager.matchPrefix != "ab" {
		t.Errorf("grid narrowed to %q, want %q", manager.matchPrefix, "ab")
	}

	if manager.gridDraws != 0 || manager.resizes != 0 || manager.visible {
		t.Error("a grid update paid for a draw or the window sequence")
	}

	cells := gridFrame(t, "").Grid.Cells()
	if len(cells) == 0 {
		t.Fatal("fixture grid has no cells")
	}

	adapter.ShowGridSubgrid(cells[0])

	if manager.subgridCell != cells[0] {
		t.Error("the subgrid's cell never reached the overlay")
	}
}

// TestAdapterUpdateGridPointer_PlacesThePointerOnTheNamedSurface pins that the
// caller says which grid surface the pointer belongs to and nothing else: its
// size and color are the Style the overlay already resolved.
func TestAdapterUpdateGridPointer_PlacesThePointerOnTheNamedSurface(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, pointerStyles{}, zap.NewNop())

	adapter.UpdateGridPointer(
		domain.ModeGrid,
		ports.GridPointer{Visible: true, Position: image.Pt(12, 34)},
	)

	if manager.pointer.mode != overlay.ModeGrid || !manager.pointer.visible {
		t.Errorf("pointer drawn as %+v, want visible on the grid surface", manager.pointer)
	}

	if manager.pointer.point != image.Pt(12, 34) {
		t.Errorf("pointer drawn at %v, want (12,34)", manager.pointer.point)
	}

	if manager.pointer.size != pointerSize || manager.pointer.fillColor != pointerFill {
		t.Errorf("pointer drawn with size %d and fill %q, want the resolved style's %d and %q",
			manager.pointer.size, manager.pointer.fillColor, pointerSize, pointerFill)
	}

	adapter.UpdateGridPointer(domain.ModeRecursiveGrid, ports.GridPointer{})

	if manager.pointer.mode != overlay.ModeRecursiveGrid || manager.pointer.visible {
		t.Errorf("pointer left as %+v, want hidden on the recursive-grid surface", manager.pointer)
	}
}

// pointerSize and pointerFill are the resolved virtual-pointer style the
// pointer test expects to arrive without the caller naming it.
const (
	pointerSize = 24
	pointerFill = "#abcdef"
)

// pointerStyles resolves only the virtual pointer's appearance.
type pointerStyles struct{}

func (pointerStyles) Apply(_ *config.Config) {}
func (pointerStyles) Refresh()               {}

func (pointerStyles) Style() overlay.Style {
	return overlay.Style{
		VirtualPointer: overlay.VirtualPointerStyle{
			FontSize:  pointerSize,
			FillColor: pointerFill,
		},
	}
}

// TestAdapterClearFrame_TakesTheFrameOffScreen is the leaving half: the
// content goes, the window goes, and the overlay is idle again — one call, so
// no caller can do two of the three and leave the last mode on screen.
func TestAdapterClearFrame_TakesTheFrameOffScreen(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	screen := image.Rect(0, 0, 1920, 1080)

	showErr := adapter.ShowFrame(
		context.Background(),
		hintsFrame(t, screen, "ab", image.Pt(10, 10)),
	)
	if showErr != nil {
		t.Fatalf("ShowFrame() error = %v", showErr)
	}

	clearErr := adapter.ClearFrame(context.Background())
	if clearErr != nil {
		t.Fatalf("ClearFrame() error = %v", clearErr)
	}

	if manager.visible {
		t.Error("the overlay is still on screen after the frame was cleared")
	}

	if manager.mode != overlay.ModeIdle {
		t.Errorf("overlay mode = %q, want %q", manager.mode, overlay.ModeIdle)
	}

	if manager.cleared == 0 {
		t.Error("the frame's content was never cleared")
	}

	if adapter.IsVisible() {
		t.Error("IsVisible() = true after the frame was cleared")
	}
}

// TestAdapterFrame_ReportsACanceledContext keeps a canceled activation from
// drawing over whatever replaced it.
func TestAdapterFrame_ReportsACanceledContext(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	frame := hintsFrame(t, image.Rect(0, 0, 1920, 1080), "ab", image.Pt(10, 10))

	showErr := adapter.ShowFrame(ctx, frame)
	if showErr == nil {
		t.Error("ShowFrame() error = nil for a canceled context")
	}

	redrawErr := adapter.RedrawFrame(ctx, frame)
	if redrawErr == nil {
		t.Error("RedrawFrame() error = nil for a canceled context")
	}

	clearErr := adapter.ClearFrame(ctx)
	if clearErr == nil {
		t.Error("ClearFrame() error = nil for a canceled context")
	}

	if manager.visible {
		t.Error("a canceled context still put the overlay on screen")
	}
}

// searchStyles is a StyleSource that resolves only the search input's
// geometry, which is the half of the Style the placement depends on.
type searchStyles struct {
	layout overlay.SearchInputLayout
}

func (searchStyles) Apply(_ *config.Config) {}
func (searchStyles) Refresh()               {}

func (s searchStyles) Style() overlay.Style {
	return overlay.Style{HintSearchLayout: s.layout}
}

// wantSearchInputBounds is where a 200x40 input with a 10/20 inset lands on a
// 1000x800 screen for each accepted anchor, worked out from the anchor's name
// rather than from the code: top_left is the insets themselves, a centered axis
// is the space left over halved and then inset the same way, and a far edge is
// the screen less the box less the inset.
//
// It is keyed by config.SearchInputPositions() and the test below fails when
// the two disagree, so a position added to the vocabulary has to be given a
// place here — and, since an anchor the placement does not recognize is now
// refused rather than drawn, a branch in the overlay to go with it.
func wantSearchInputBounds() map[string]image.Rectangle {
	return map[string]image.Rectangle{
		config.SearchInputPositionTopLeft:      image.Rect(10, 20, 210, 60),
		config.SearchInputPositionTopCenter:    image.Rect(410, 20, 610, 60),
		config.SearchInputPositionTopRight:     image.Rect(790, 20, 990, 60),
		config.SearchInputPositionCenter:       image.Rect(410, 400, 610, 440),
		config.SearchInputPositionBottomLeft:   image.Rect(10, 740, 210, 780),
		config.SearchInputPositionBottomCenter: image.Rect(410, 740, 610, 780),
		config.SearchInputPositionBottomRight:  image.Rect(790, 740, 990, 780),
	}
}

// TestAdapterHintSearchBounds_PlacesTheInputFromConfiguration pins where the
// search input lands for each accepted anchor. The maths moved out of the mode
// layer with #1210 — a caller says which screen, and the overlay says where on
// it, because the IME field has to be put over the same rectangle.
//
// It walks config.SearchInputPositions() rather than a list of its own, so an
// anchor the validator accepts and the overlay does not place fails here.
func TestAdapterHintSearchBounds_PlacesTheInputFromConfiguration(t *testing.T) {
	t.Parallel()

	screen := image.Rect(0, 0, 1000, 800)

	layout := overlay.SearchInputLayout{
		Width:   200,
		Height:  40,
		XOffset: 10,
		YOffset: 20,
	}

	positions := config.SearchInputPositions()
	if len(positions) == 0 {
		t.Fatal("config.SearchInputPositions() is empty; there is no vocabulary to place")
	}

	wanted := wantSearchInputBounds()
	if len(wanted) != len(positions) {
		t.Errorf("%d placements expected for %d accepted positions",
			len(wanted), len(positions))
	}

	for _, position := range positions {
		t.Run(position, func(t *testing.T) {
			t.Parallel()

			want, expected := wanted[position]
			if !expected {
				t.Fatalf("%q is accepted but no placement is expected for it", position)
			}

			styles := searchStyles{layout: layout}
			styles.layout.Position = position

			adapter := overlay.NewAdapter(newScreenManager(), styles, zap.NewNop())

			if got := adapter.HintSearchBounds(screen); got != want {
				t.Errorf("HintSearchBounds() = %v, want %v", got, want)
			}
		})
	}
}

// TestAdapterHintSearchBounds_KeepsTheInputOnScreen pins the clamp: an offset
// larger than the display cannot push the input off it, because a search box
// the user cannot see is a search they cannot cancel.
func TestAdapterHintSearchBounds_KeepsTheInputOnScreen(t *testing.T) {
	t.Parallel()

	styles := searchStyles{layout: overlay.SearchInputLayout{
		Position: config.SearchInputPositionTopLeft,
		Width:    200,
		Height:   40,
		XOffset:  5000,
		YOffset:  5000,
	}}

	adapter := overlay.NewAdapter(newScreenManager(), styles, zap.NewNop())

	screen := image.Rect(0, 0, 1000, 800)

	got := adapter.HintSearchBounds(screen)
	if want := image.Rect(800, 760, 1000, 800); got != want {
		t.Errorf("HintSearchBounds() = %v, want %v", got, want)
	}
}

// TestAdapterDrawHintSearch_PutsTheQueryAndCountWhereTheStyleSaid is the
// search input's half of the same move: the caller says what is in the box and
// which screen it is on, and the overlay decides where the box goes.
func TestAdapterDrawHintSearch_PutsTheQueryAndCountWhereTheStyleSaid(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	styles := searchStyles{layout: overlay.SearchInputLayout{
		Position: config.SearchInputPositionTopLeft,
		Width:    200,
		Height:   40,
		XOffset:  10,
		YOffset:  20,
	}}

	adapter := overlay.NewAdapter(manager, styles, zap.NewNop())

	err := adapter.DrawHintSearch(ports.HintSearch{
		Screen:      image.Rect(0, 0, 1000, 800),
		Query:       "sett",
		ResultCount: 3,
	})
	if err != nil {
		t.Fatalf("DrawHintSearch() error = %v", err)
	}

	if manager.searchQuery != "sett" {
		t.Errorf("query drawn = %q, want %q", manager.searchQuery, "sett")
	}

	if manager.searchResults != 3 {
		t.Errorf("result count drawn = %d, want 3", manager.searchResults)
	}

	if got := manager.searchFrame.Position(); got != image.Pt(10, 20) {
		t.Errorf("search input drawn at %v, want %v", got, image.Pt(10, 20))
	}

	if got := manager.searchFrame.Width(); got != 200 {
		t.Errorf("search input width = %d, want 200", got)
	}

	adapter.HideHintSearch()

	if manager.searchHides != 1 {
		t.Errorf("search input hidden %d times, want 1", manager.searchHides)
	}
}

// unknownSearchAnchor is a `hints.search_input_ui.position` value the vocabulary
// does not declare — the shape an eighth anchor would have on the day someone
// adds it to config and forgets the overlay.
const unknownSearchAnchor = "floating"

// anchoredSearchStyles is a resolved search-input geometry that differs from
// the next one only in where it is anchored, which is the whole of what the
// refusal tests below vary.
func anchoredSearchStyles(position string) searchStyles {
	return searchStyles{layout: overlay.SearchInputLayout{
		Position: position,
		Width:    200,
		Height:   40,
	}}
}

// TestAdapterDrawHintSearch_UnknownAnchorIsRefusedNotDrawn pins the other half
// of the vocabulary: an anchor the placement has no branch for is refused
// before anything is drawn, rather than being placed in the top-left corner.
// Top-left used to be where an unrecognized anchor landed, which made an anchor
// nobody implemented indistinguishable from `top_left` — silent everywhere and
// wrong on screen.
func TestAdapterDrawHintSearch_UnknownAnchorIsRefusedNotDrawn(t *testing.T) {
	t.Parallel()

	screen := image.Rect(0, 0, 1000, 800)

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, anchoredSearchStyles(unknownSearchAnchor), zap.NewNop())

	err := adapter.DrawHintSearch(ports.HintSearch{
		Screen:      screen,
		Query:       "abc",
		ResultCount: 3,
	})
	if !derrors.IsNotSupported(err) {
		t.Errorf(
			"DrawHintSearch() with an unrecognized anchor = %v (code %q), want CodeNotSupported",
			err, derrors.GetCode(err),
		)
	}

	if manager.searchDraws != 0 {
		t.Errorf(
			"search input drawn %d times for an unrecognized anchor, want 0: "+
				"it would have been drawn in the top-left corner",
			manager.searchDraws,
		)
	}

	// The vocabulary's own anchors still draw. A guard that refused everything
	// would pass the assertion above and leave every user with no search box.
	for _, position := range config.SearchInputPositions() {
		drawn := newScreenManager()

		drawErr := overlay.NewAdapter(drawn, anchoredSearchStyles(position), zap.NewNop()).
			DrawHintSearch(ports.HintSearch{Screen: screen})
		if drawErr != nil {
			t.Errorf("DrawHintSearch() with %q = %v, want nil", position, drawErr)
		}

		if drawn.searchDraws != 1 {
			t.Errorf("search input drawn %d times for %q, want 1", drawn.searchDraws, position)
		}
	}
}

// TestAdapterHintSearchBounds_UnknownAnchorHasNoBounds pins how the bounds half
// degrades. It answers a rectangle and cannot report, so an anchor the
// placement refuses answers the empty one — the same nothing a handler with no
// overlay at all already returns, and honest, because the draw refused too and
// there is no box on screen for the IME field to be put over.
func TestAdapterHintSearchBounds_UnknownAnchorHasNoBounds(t *testing.T) {
	t.Parallel()

	adapter := overlay.NewAdapter(
		newScreenManager(),
		anchoredSearchStyles(unknownSearchAnchor),
		zap.NewNop(),
	)

	if got := adapter.HintSearchBounds(image.Rect(0, 0, 1000, 800)); !got.Empty() {
		t.Errorf(
			"HintSearchBounds() with an unrecognized anchor = %v, want the empty rectangle",
			got,
		)
	}
}

// TestAdapterSearchInput_UnsetAnchorIsRefusedWithTheRest pins the empty string
// as unrecognized rather than as a default, and deliberately so. Unlike
// `hints.ui.placement`, an unset `hints.search_input_ui.position` is not settled
// anywhere: the validator refuses it (config/search_input_position.go), so the
// only Style that carries one is a Style nothing ever resolved — whose width and
// height are zero too, and which would draw a box of no size wherever it landed.
func TestAdapterSearchInput_UnsetAnchorIsRefusedWithTheRest(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, searchStyles{}, zap.NewNop())

	screen := image.Rect(0, 0, 1000, 800)

	err := adapter.DrawHintSearch(ports.HintSearch{Screen: screen})
	if !derrors.IsNotSupported(err) {
		t.Errorf(
			"DrawHintSearch() with an unset anchor = %v (code %q), want CodeNotSupported",
			err, derrors.GetCode(err),
		)
	}

	if manager.searchDraws != 0 {
		t.Errorf("search input drawn %d times for an unset anchor, want 0", manager.searchDraws)
	}

	if got := adapter.HintSearchBounds(screen); !got.Empty() {
		t.Errorf("HintSearchBounds() with an unset anchor = %v, want the empty rectangle", got)
	}
}

// monitorSelectManager is a backend that declares the optional monitor-select
// capability, the way the darwin and Linux ones do.
type monitorSelectManager struct {
	*screenManager

	panels []overlay.MonitorSelectTarget
	style  overlay.MonitorSelectStyle
	draws  int
	hides  int
}

func newMonitorSelectManager() *monitorSelectManager {
	return &monitorSelectManager{screenManager: newScreenManager()}
}

func (m *monitorSelectManager) DrawMonitorSelect(
	targets []overlay.MonitorSelectTarget,
	style overlay.MonitorSelectStyle,
) error {
	m.panels = targets
	m.style = style
	m.draws++

	return nil
}

func (m *monitorSelectManager) HideMonitorSelect() { m.hides++ }

// monitorSelectStyles resolves a monitor-select Style, so a test can tell that
// the panels were met with the appearance the overlay owns rather than with
// one a caller carried.
type monitorSelectStyles struct{}

func (monitorSelectStyles) Apply(_ *config.Config) {}
func (monitorSelectStyles) Refresh()               {}

func (monitorSelectStyles) Style() overlay.Style {
	return overlay.Style{
		MonitorSelect: overlay.MonitorSelectStyle{FontSize: 42, TextColor: "#abcdef"},
	}
}

// TestAdapterShowFrame_PutsTheMonitorPickerOnScreen is the acceptance for the
// last surface to convert: a mode says which displays are on offer, and the
// overlay meets that with the Style it resolved and the capability the backend
// declared.
func TestAdapterShowFrame_PutsTheMonitorPickerOnScreen(t *testing.T) {
	t.Parallel()

	manager := newMonitorSelectManager()
	adapter := overlay.NewAdapter(manager, monitorSelectStyles{}, zap.NewNop())

	second := image.Rect(1920, 0, 3840, 1080)

	showErr := adapter.ShowFrame(context.Background(), ports.MonitorSelectFrame{
		Targets: []ports.MonitorSelectTarget{
			{
				Bounds:           second,
				Label:            "b",
				Name:             "SecondDisplay",
				Selected:         true,
				MatchedPrefixLen: 1,
			},
		},
	})
	if showErr != nil {
		t.Fatalf("ShowFrame() error = %v", showErr)
	}

	if manager.mode != overlay.ModeMonitorSelect {
		t.Errorf("overlay mode = %q, want %q", manager.mode, overlay.ModeMonitorSelect)
	}

	if len(manager.panels) != 1 {
		t.Fatalf("panels drawn = %d, want 1", len(manager.panels))
	}

	panel := manager.panels[0]
	if panel.Bounds != second || panel.Label != "b" || panel.Subtitle != "SecondDisplay" {
		t.Errorf("panel drawn = %+v, want the display the frame named", panel)
	}

	if !panel.Selected || panel.MatchedPrefixLen != 1 {
		t.Errorf("panel drawn = %+v, want the selection and match the frame carried", panel)
	}

	if manager.style.FontSize != 42 {
		t.Errorf("panel style = %+v, want the Style the overlay resolved", manager.style)
	}

	// The picker draws on panels of its own, so the shared overlay window is
	// left alone rather than brought up empty behind them.
	if manager.visible {
		t.Error("the shared overlay window was shown for a surface that does not draw on it")
	}
}

// TestAdapterShowFrame_ReportsAMonitorPickerTheBackendCannotDraw pins the
// extension pattern: drawing the picker is an optional capability reached by
// type assertion, and a backend without it degrades rather than fails.
func TestAdapterShowFrame_ReportsAMonitorPickerTheBackendCannotDraw(t *testing.T) {
	t.Parallel()

	adapter := overlay.NewAdapter(newScreenManager(), testStyles{}, zap.NewNop())

	showErr := adapter.ShowFrame(context.Background(), ports.MonitorSelectFrame{
		Targets: []ports.MonitorSelectTarget{{Bounds: image.Rect(0, 0, 10, 10), Label: "a"}},
	})
	if !derrors.IsNotSupported(showErr) {
		t.Fatalf("ShowFrame() error = %v, want CodeNotSupported", showErr)
	}
}

// TestAdapterClearFrame_TakesTheMonitorPanelsDown covers the failure the
// Indicator work exists to prevent, one surface further: the panels are not on
// the shared surface, so clearing that surface is not enough to take them off
// the screen.
func TestAdapterClearFrame_TakesTheMonitorPanelsDown(t *testing.T) {
	t.Parallel()

	manager := newMonitorSelectManager()
	adapter := overlay.NewAdapter(manager, monitorSelectStyles{}, zap.NewNop())

	clearErr := adapter.ClearFrame(context.Background())
	if clearErr != nil {
		t.Fatalf("ClearFrame() error = %v", clearErr)
	}

	if manager.hides != 0 {
		t.Errorf("panels hidden %d times before any were drawn, want 0", manager.hides)
	}

	showErr := adapter.ShowFrame(context.Background(), ports.MonitorSelectFrame{
		Targets: []ports.MonitorSelectTarget{{Bounds: image.Rect(0, 0, 10, 10), Label: "a"}},
	})
	if showErr != nil {
		t.Fatalf("ShowFrame() error = %v", showErr)
	}

	clearErr = adapter.ClearFrame(context.Background())
	if clearErr != nil {
		t.Fatalf("ClearFrame() error = %v", clearErr)
	}

	if manager.hides != 1 {
		t.Errorf("panels hidden %d times after being drawn, want 1", manager.hides)
	}
}

// TestAdapterShowFrame_ScrollTakesTheSurfaceOverWithoutDrawing pins what
// entering scroll mode means for the overlay: the mode the indicators name
// changes and whatever the previous mode drew comes off the surface, but
// nothing of scroll's own is drawn on it.
//
// The window still comes up, because on Linux the indicators that report the
// mode are painted on that surface — a hidden window is an invisible
// indicator.
func TestAdapterShowFrame_ScrollTakesTheSurfaceOverWithoutDrawing(t *testing.T) {
	t.Parallel()

	manager := newScreenManager()
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	showErr := adapter.ShowFrame(context.Background(), ports.ScrollFrame{})
	if showErr != nil {
		t.Fatalf("ShowFrame() error = %v", showErr)
	}

	if manager.mode != overlay.ModeScroll {
		t.Errorf("overlay mode = %q, want %q", manager.mode, overlay.ModeScroll)
	}

	if manager.cleared != 1 {
		t.Errorf("surface cleared %d times, want 1", manager.cleared)
	}

	if !manager.visible {
		t.Error("the overlay the mode indicator is painted on was left hidden")
	}

	if manager.gridDraws != 0 || len(manager.drawn) != 0 {
		t.Error("scroll drew content of its own on the shared surface")
	}
}
