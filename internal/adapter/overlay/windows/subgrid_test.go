//go:build windows

package windows

import (
	"image"
	"slices"
	"testing"

	"go.uber.org/zap"
	"golang.org/x/sys/windows"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// recordingWindow is an overlayWindow that paints nothing and remembers what
// it was asked to paint. It stands in for the layered HWND so these tests need
// no desktop, and it observes the grid overlay at the one boundary the window
// sees. It is always healthy and on screen: every repaint under test is one
// the real window would accept.
type recordingWindow struct {
	// texts is every string painted, in order, the pointer glyph included:
	// the glyph is a label like any other on this surface, and a screen is
	// the list of them.
	texts []string
	// rects is every rectangle filled or stroked, in order.
	rects []image.Rectangle
}

func (w *recordingWindow) HWND() windows.HWND { return 0 }

func (w *recordingWindow) Healthy() bool { return true }

func (w *recordingWindow) Visible() bool { return true }

func (w *recordingWindow) Bounds() image.Rectangle { return image.Rect(0, 0, 800, 600) }

func (w *recordingWindow) Backend() string { return "recording" }

func (w *recordingWindow) Show() {}

func (w *recordingWindow) Hide() {}

func (w *recordingWindow) Clear() {}

func (w *recordingWindow) ResizeToActiveScreen() error { return nil }

func (w *recordingWindow) Destroy() {}

func (w *recordingWindow) FillRect(bounds image.Rectangle, _ uint32) {
	w.rects = append(w.rects, bounds)
}

func (w *recordingWindow) StrokeRect(bounds image.Rectangle, _ uint32, _ float64) {
	w.rects = append(w.rects, bounds)
}

func (w *recordingWindow) FillRoundedRect(bounds image.Rectangle, _ float64, _ uint32) {
	w.rects = append(w.rects, bounds)
}

func (w *recordingWindow) StrokeRoundedRect(
	bounds image.Rectangle,
	_ float64,
	_ uint32,
	_ float64,
) {
	w.rects = append(w.rects, bounds)
}

func (w *recordingWindow) FillTriangle(_, _, _ image.Point, _ uint32) {}

func (w *recordingWindow) DrawTextCentered(
	text string,
	_ image.Rectangle,
	_ string,
	_ float64,
	_ uint32,
) {
	w.texts = append(w.texts, text)
}

func (w *recordingWindow) DrawPointerGlyph(_ image.Point, _ int, char string, _ string, _ uint32) {
	w.texts = append(w.texts, char)
}

func (w *recordingWindow) Flush() error { return nil }

// forget drops everything painted so far, so what a test asserts on is what
// the call under test painted.
func (w *recordingWindow) forget() {
	w.texts = nil
	w.rects = nil
}

// fixedTheme is a ThemeProvider that answers one mode.
type fixedTheme bool

func (t fixedTheme) IsDarkMode() bool { return bool(t) }

// gridPointerAppearance is a pointer look no default could produce, so a draw
// that reads it cannot be mistaken for a draw that fell back.
var gridPointerAppearance = manager.PointerAppearance{
	FillColor:  "#123456",
	FontFamily: "Test Sans",
	Char:       "✛",
	FontSize:   20,
}

// subgridTestKeys is a key per cell of the subgrid every overlay draws. The
// test below sets it on the surface directly because this manager is stood up
// without render components, and a manager with none leaves the keys it was
// last given alone (syncSublayerKeysLocked). The labels drawn from them are
// upper-cased, as every grid label is.
const subgridTestKeys = "abcdefghi"

// gridPointerAt is the pointer standing at point, wearing the appearance above.
func gridPointerAt(point image.Point) recursivegridcomponent.VirtualPointerState {
	return recursivegridcomponent.VirtualPointerState{
		Visible:   true,
		Position:  point,
		Size:      gridPointerAppearance.FontSize,
		FillColor: gridPointerAppearance.FillColor,
		Char:      gridPointerAppearance.Char,
		FontName:  gridPointerAppearance.FontFamily,
	}
}

// testGrid is a grid the size of the recording window, which is what a draw
// and a subgrid open both take.
func testGrid() *domainGrid.Grid {
	return domainGrid.NewGrid("ab", image.Rect(0, 0, 800, 600), zap.NewNop())
}

// gridOnWindow stands a Manager up over a recording window, draws a labeled
// grid on it and forgets everything that took, so what a test asserts on is
// what the calls after it painted.
func gridOnWindow(t *testing.T) (*Manager, *recordingWindow) {
	t.Helper()

	window := &recordingWindow{}
	overlayManager := &Manager{Base: manager.NewBase(zap.NewNop())}
	overlayManager.win = &winOverlay{window: window, renderMu: &overlayManager.renderMu}

	style := gridcomponent.BuildStyle(config.DefaultConfig().Grid, fixedTheme(false))

	err := overlayManager.DrawGrid(testGrid(), "", style)
	if err != nil {
		t.Fatalf("DrawGrid() error = %v", err)
	}

	window.forget()

	return overlayManager, window
}

// TestWindowsOverlayManager_ShowSubgrid_IsWhatEveryRepaintAfterItDraws pins
// what #1610 asks for, the way the Linux test of the same name pins #1491:
// that the open shows the subgrid alone, which is the macOS answer, and that
// every later repaint of the same surface shows exactly what the open left,
// because nothing between them is anything the user typed. It used to hold
// for the pointer's repaint and not for the narrowing one, which put the
// parent cells back underneath the subgrid on the first keystroke after an
// open, and hide-unmatched thinned those cells rather than removing them.
//
// wantSubgridOnly is a spelt-out screen, so an open that also drew the parent
// cells fails even though every repaint after it agreed. Comparing the
// rectangles themselves rather than counting them extends the second half to
// geometry: a repaint that drew the subgrid somewhere else, or at another
// size, fails too.
func TestWindowsOverlayManager_ShowSubgrid_IsWhatEveryRepaintAfterItDraws(t *testing.T) {
	t.Parallel()

	// One label per subgridTestKeys character, and then the pointer stand-in
	// that rides the same pass. A parent cell label ("AA" and its fifteen
	// siblings) appearing anywhere in this list is the defect.
	wantSubgridOnly := []string{
		"A", "B", "C", "D", "E", "F", "G", "H", "I",
		gridPointerAppearance.Char,
	}

	repaints := []struct {
		name    string
		repaint func(*Manager)
	}{
		{
			name: "a narrowing keystroke",
			repaint: func(overlayManager *Manager) {
				overlayManager.UpdateGridMatches("a")
			},
		},
		{
			name: "the pointer moving",
			repaint: func(overlayManager *Manager) {
				overlayManager.DrawGridPointer(
					manager.ModeGrid,
					image.Pt(300, 300),
					gridPointerAppearance,
				)
			},
		},
		{
			// The toggle paints nothing by itself; the next repaint reads it.
			name: "hide-unmatched toggled before a narrowing keystroke",
			repaint: func(overlayManager *Manager) {
				overlayManager.SetHideUnmatched(true)
				overlayManager.UpdateGridMatches("a")
			},
		},
	}

	for _, testCase := range repaints {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			overlayManager, window := gridOnWindow(t)
			overlayManager.win.sublayerKeys = subgridTestKeys

			overlayManager.ShowSubgrid(
				testGrid().AllCells()[0],
				gridcomponent.Style{},
				gridPointerAt(image.Pt(120, 240)),
			)

			opened := slices.Clone(window.texts)
			openedRects := slices.Clone(window.rects)

			if !slices.Equal(opened, wantSubgridOnly) {
				t.Fatalf("the open painted %v, want %v, the subgrid and nothing under it",
					opened, wantSubgridOnly)
			}

			window.forget()

			testCase.repaint(overlayManager)

			if !slices.Equal(window.texts, opened) {
				t.Errorf("the repaint painted %v, want the %v the open left",
					window.texts, opened)
			}

			if !slices.Equal(window.rects, openedRects) {
				t.Errorf("the repaint drew %d rectangles, want the %d the open left, unmoved",
					len(window.rects), len(openedRects))
			}
		})
	}
}
