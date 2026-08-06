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
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/ports"
)

// testStyles is a StyleSource for the health tests, which never draw.
type testStyles struct{}

func (testStyles) Style() overlay.Style { return overlay.Style{} }

type supportedManager struct {
	overlay.NoOpManager
}

type stubManager struct {
	overlay.NoOpManager
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
		&overlay.NoOpManager{},
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
	overlay.NoOpManager

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
	cols     int
	rows     int
	nextKeys string
	nextCols int
	nextRows int
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
	gridCols int,
	gridRows int,
	nextKeys string,
	nextGridCols int,
	nextGridRows int,
	_ renderrecursivegrid.Style,
	virtualPointer renderrecursivegrid.VirtualPointerState,
) error {
	m.recursiveGrid = recursiveGridDraw{
		bounds:   bounds,
		depth:    depth,
		keys:     keys,
		cols:     gridCols,
		rows:     gridRows,
		nextKeys: nextKeys,
		nextCols: nextGridCols,
		nextRows: nextGridRows,
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

	frame := ports.RecursiveGridFrame{
		Bounds:     image.Rect(10, 20, 210, 220),
		Depth:      2,
		Layout:     ports.RecursiveGridLayout{Keys: "qwer", GridCols: 2, GridRows: 2},
		NextLayout: ports.RecursiveGridLayout{Keys: "asdf", GridCols: 2, GridRows: 2},
		Pointer:    ports.GridPointer{Visible: true, Position: image.Pt(30, 40)},
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

	if drawn.cols != frame.Layout.GridCols || drawn.rows != frame.Layout.GridRows {
		t.Errorf("recursive grid drawn %dx%d, want %dx%d",
			drawn.cols, drawn.rows, frame.Layout.GridCols, frame.Layout.GridRows)
	}

	if drawn.nextKeys != frame.NextLayout.Keys || drawn.nextCols != frame.NextLayout.GridCols ||
		drawn.nextRows != frame.NextLayout.GridRows {
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
		Layout: ports.RecursiveGridLayout{Keys: "qwer", GridCols: 2, GridRows: 2},
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

// searchInputTopLeft is the anchor the placement tests start from: the one
// where the configured offsets are the position, with no edge maths.
const searchInputTopLeft renderhints.SearchInputPosition = "top_left"

// searchStyles is a StyleSource that resolves only the search input's
// geometry, which is the half of the Style the placement depends on.
type searchStyles struct {
	layout overlay.SearchInputLayout
}

func (s searchStyles) Style() overlay.Style {
	return overlay.Style{HintSearchLayout: s.layout}
}

// TestAdapterHintSearchBounds_PlacesTheInputFromConfiguration pins where the
// search input lands for each configured anchor. The maths moved out of the
// mode layer with #1210 — a caller says which screen, and the overlay says
// where on it, because the IME field has to be put over the same rectangle.
func TestAdapterHintSearchBounds_PlacesTheInputFromConfiguration(t *testing.T) {
	t.Parallel()

	screen := image.Rect(0, 0, 1000, 800)

	layout := overlay.SearchInputLayout{
		Width:   200,
		Height:  40,
		XOffset: 10,
		YOffset: 20,
	}

	tests := []struct {
		name     string
		position renderhints.SearchInputPosition
		want     image.Rectangle
	}{
		{
			name:     "top left is the offsets themselves",
			position: searchInputTopLeft,
			want:     image.Rect(10, 20, 210, 60),
		},
		{
			name:     "top center centers horizontally",
			position: "top_center",
			want:     image.Rect(410, 20, 610, 60),
		},
		{
			name:     "top right measures from the right edge",
			position: "top_right",
			want:     image.Rect(790, 20, 990, 60),
		},
		{
			name:     "center centers on both axes",
			position: "center",
			want:     image.Rect(410, 400, 610, 440),
		},
		{
			name:     "bottom right measures from both far edges",
			position: "bottom_right",
			want:     image.Rect(790, 740, 990, 780),
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			styles := searchStyles{layout: layout}
			styles.layout.Position = testCase.position

			adapter := overlay.NewAdapter(newScreenManager(), styles, zap.NewNop())

			if got := adapter.HintSearchBounds(screen); got != testCase.want {
				t.Errorf("HintSearchBounds() = %v, want %v", got, testCase.want)
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
		Position: searchInputTopLeft,
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
		Position: searchInputTopLeft,
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
