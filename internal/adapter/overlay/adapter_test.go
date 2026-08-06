package overlay_test

import (
	"context"
	"image"
	"slices"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	renderhints "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/domain/element"
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
