//go:build linux && cgo

package linux

import (
	"image"
	"slices"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// gridPointerAppearance is a pointer look no default could produce, so a draw
// that reads it cannot be mistaken for a draw that fell back.
var gridPointerAppearance = manager.PointerAppearance{
	FillColor:  "#123456",
	FontFamily: "Test Sans",
	Char:       "✛",
	FontSize:   20,
}

// noGridPointer is what a draw that carries a pointer carries when the real
// cursor is on the selection and there is nothing to stand in for.
var noGridPointer recursivegridcomponent.VirtualPointerState

// gridPointerAt is the pointer standing at point, wearing the appearance above
// so that a draw which reads it cannot be mistaken for one that fell back.
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

// gridOnSurface draws a grid on a recording surface and forgets everything
// that took, so what a test asserts on is what the pointer call painted.
func gridOnSurface(t *testing.T) (*Manager, *recordingSurface) {
	t.Helper()

	overlayManager, surface := recordingManager()

	err := overlayManager.DrawGrid(
		domainGrid.NewGrid("ab", image.Rect(0, 0, 800, 600), zap.NewNop()),
		"",
		gridcomponent.Style{},
	)
	if err != nil {
		t.Fatalf("DrawGrid() error = %v", err)
	}

	surface.forget()

	return overlayManager, surface
}

// TestLinuxOverlayManager_DrawGridPointer_PaintsTheConfiguredGlyphWithTheGrid
// is the whole of what #1463 asks for: grid mode's pointer stand-in is drawn
// on Linux, as recursive grid's already is. It rides the grid's own repaint
// rather than a window of its own, so the assertion is that the glyph reaches
// the surface at all — and that it is the one the user configured, since the
// appearance now travels with the position.
func TestLinuxOverlayManager_DrawGridPointer_PaintsTheConfiguredGlyphWithTheGrid(t *testing.T) {
	t.Parallel()

	overlayManager, surface := gridOnSurface(t)

	overlayManager.DrawGridPointer(
		manager.ModeGrid,
		image.Pt(120, 240),
		gridPointerAppearance,
	)

	painted, found := surface.findText(gridPointerAppearance.Char)
	if !found {
		t.Fatalf("painted %v, want the pointer glyph among them", surface.paintedStrings())
	}

	if painted.center != image.Pt(120, 240) {
		t.Errorf("pointer centered at %v, want the position the mode named", painted.center)
	}

	if painted.fontFamily != gridPointerAppearance.FontFamily {
		t.Errorf("pointer drawn in %q, want the resolved family %q",
			painted.fontFamily, gridPointerAppearance.FontFamily)
	}

	// The surface reports scale 1 here, so the drawn size is the configured
	// one; a backend that scaled it would report otherwise.
	if painted.fontSize != float64(gridPointerAppearance.FontSize) {
		t.Errorf("pointer drawn at %v points, want the resolved %d",
			painted.fontSize, gridPointerAppearance.FontSize)
	}

	if painted.color != 0xFF123456 {
		t.Errorf("pointer fill = %#08x, want the resolved #123456", painted.color)
	}
}

// The pointer is screen-local like the cells it sits among, so it belongs on
// the monitor the grid was placed on. Getting this wrong puts the pointer on
// the primary display while the grid is on the second one.
func TestLinuxOverlayManager_DrawGridPointer_PlacesThePointerOnTheActiveScreen(t *testing.T) {
	t.Parallel()

	overlayManager, surface := gridOnSurface(t)
	overlayManager.SetActiveScreenOrigin(image.Pt(1920, 0))

	overlayManager.DrawGridPointer(
		manager.ModeGrid,
		image.Pt(120, 240),
		gridPointerAppearance,
	)

	painted, found := surface.findText(gridPointerAppearance.Char)
	if !found {
		t.Fatalf("painted %v, want the pointer glyph among them", surface.paintedStrings())
	}

	if painted.center != image.Pt(2040, 240) {
		t.Errorf("pointer centered at %v, want it translated onto the active monitor",
			painted.center)
	}
}

// Teardown: hiding takes the glyph off the surface and leaves the grid on it.
// Grid mode's exit hides the pointer before the frame is cleared, so a repaint
// that still carried the pointer would leave it on screen over the next mode.
func TestLinuxOverlayManager_HideGridPointer_RepaintsTheGridWithoutIt(t *testing.T) {
	t.Parallel()

	overlayManager, surface := gridOnSurface(t)

	overlayManager.DrawGridPointer(
		manager.ModeGrid,
		image.Pt(120, 240),
		gridPointerAppearance,
	)
	surface.forget()

	overlayManager.HideGridPointer(manager.ModeGrid)

	if surface.paintedText(gridPointerAppearance.Char) {
		t.Error("the pointer glyph was painted again after the mode hid it")
	}

	if !surface.paintedText("AAAA") {
		t.Errorf("painted %v, want the grid's own cells back", surface.paintedStrings())
	}
}

// Narrowing is the per-keystroke path and it must keep drawing the pointer:
// the mode sets it once when a cell is chosen and then only repaints, so a
// redraw that forgot it would blink the pointer out on the next key.
func TestLinuxOverlayManager_UpdateGridMatches_KeepsThePointerOnTheSurface(t *testing.T) {
	t.Parallel()

	overlayManager, surface := gridOnSurface(t)

	overlayManager.DrawGridPointer(
		manager.ModeGrid,
		image.Pt(120, 240),
		gridPointerAppearance,
	)
	surface.forget()

	overlayManager.UpdateGridMatches("a")

	if !surface.paintedText(gridPointerAppearance.Char) {
		t.Errorf("painted %v, want the pointer still among them", surface.paintedStrings())
	}
}

// A subgrid replaces the cells on the same surface and the pointer stands on
// the cell it was opened inside, so the two are one keystroke's worth of
// change — and #1492 is that this surface must be repainted once for it.
//
// The pointer starts somewhere else and the open moves it, which is what makes
// the flush count an assertion rather than an accident: a backend that ignored
// the pointer it was handed would need a second repaint to catch up, and one
// that took it would paint the glyph at its new place in the frame it was
// already painting.
func TestLinuxOverlayManager_ShowSubgrid_PaintsThePointerItCarriesInOneRepaint(t *testing.T) {
	t.Parallel()

	overlayManager, surface := gridOnSurface(t)
	overlayManager.x11.sublayerKeys = subgridTestKeys

	overlayManager.DrawGridPointer(
		manager.ModeGrid,
		image.Pt(10, 20),
		gridPointerAppearance,
	)
	surface.forget()

	overlayManager.ShowSubgrid(
		firstGridCell(),
		gridcomponent.Style{},
		gridPointerAt(image.Pt(120, 240)),
	)

	if surface.flushes != 1 {
		t.Errorf("the selection keystroke flushed the surface %d times, want 1", surface.flushes)
	}

	painted, found := surface.findText(gridPointerAppearance.Char)
	if !found {
		t.Fatalf("painted %v, want the pointer among them", surface.paintedStrings())
	}

	if painted.center != image.Pt(120, 240) {
		t.Errorf("pointer centered at %v, want the place the selection moved to",
			painted.center)
	}
}

// The other half of the same call: a session whose real cursor follows the
// selection has no pointer to stand in for it, and the open says so rather than
// leaving whatever the surface last held.
func TestLinuxOverlayManager_ShowSubgrid_TakesThePointerOffWhenItCarriesNone(t *testing.T) {
	t.Parallel()

	overlayManager, surface := gridOnSurface(t)
	overlayManager.x11.sublayerKeys = subgridTestKeys

	overlayManager.DrawGridPointer(
		manager.ModeGrid,
		image.Pt(120, 240),
		gridPointerAppearance,
	)
	surface.forget()

	overlayManager.ShowSubgrid(firstGridCell(), gridcomponent.Style{}, noGridPointer)

	if surface.paintedText(gridPointerAppearance.Char) {
		t.Errorf("painted %v, want no pointer: the open carried none",
			surface.paintedStrings())
	}
}

// Moving the pointer while a subgrid is open repaints the subgrid, and the
// order it does that in decides which Wayland buffer the clear lands in:
// beginFrame is what selects the writable one, so clearing before it wipes the
// buffer already on screen and shows the stale one next.
func TestLinuxOverlayManager_DrawGridPointer_BeginsTheFrameBeforeClearingIt(t *testing.T) {
	t.Parallel()

	overlayManager, surface := gridOnSurface(t)

	cell := domainGrid.NewGrid("ab", image.Rect(0, 0, 800, 600), zap.NewNop()).AllCells()[0]
	overlayManager.ShowSubgrid(cell, gridcomponent.Style{}, noGridPointer)
	surface.forget()

	overlayManager.DrawGridPointer(
		manager.ModeGrid,
		image.Pt(120, 240),
		gridPointerAppearance,
	)

	want := []string{"beginFrame", "surfaceClear", "surfaceFlush"}
	if !slices.Equal(surface.frameOps, want) {
		t.Errorf("frame ran %v, want %v", surface.frameOps, want)
	}
}

// Recursive grid keeps carrying its pointer inside the frame it hands over, so
// naming that surface here must not start a second, competing repaint of a
// grid that mode never drew.
func TestLinuxOverlayManager_DrawGridPointer_LeavesTheRecursiveGridSurfaceAlone(t *testing.T) {
	t.Parallel()

	overlayManager, surface := gridOnSurface(t)

	overlayManager.DrawGridPointer(
		manager.ModeRecursiveGrid,
		image.Pt(120, 240),
		gridPointerAppearance,
	)

	if len(surface.texts) != 0 || surface.flushes != 0 {
		t.Errorf("recursive grid's pointer painted %v and flushed %d times, want neither",
			surface.paintedStrings(), surface.flushes)
	}
}

// The teardown order the overlay's leaving half runs (#1492): clear the
// surface, then hide the pointer. The hide has to be free, because it lands on
// a surface that has just been cleared and is one call from being hidden — a
// repaint there would paint the grid, or an open subgrid, back onto it.
//
// What makes it free is the clear having already forgotten the pointer, so the
// equality guard in SetGridPointer has nothing to do. Asserting the flush count
// rather than the absence of the glyph is the point: a repaint that painted no
// pointer would still have repainted.
func TestLinuxOverlayManager_HideGridPointer_PaintsNothingAfterAClear(t *testing.T) {
	t.Parallel()

	overlayManager, surface := gridOnSurface(t)

	overlayManager.DrawGridPointer(
		manager.ModeGrid,
		image.Pt(120, 240),
		gridPointerAppearance,
	)
	overlayManager.Clear()
	surface.forget()

	overlayManager.HideGridPointer(manager.ModeGrid)

	if surface.flushes != 0 {
		t.Errorf("hiding the pointer after a clear flushed the surface %d times, want 0",
			surface.flushes)
	}

	if len(surface.paintedStrings()) != 0 {
		t.Errorf("painted %v onto a surface that was just cleared, want nothing",
			surface.paintedStrings())
	}
}

// Clearing the surface takes the grid and its pointer with it, so the record
// of what is on screen goes too: a stale one would put the pointer back on the
// first repaint of the next session.
func TestLinuxOverlayManager_Clear_ForgetsThePointerItWasDrawing(t *testing.T) {
	t.Parallel()

	overlayManager, surface := gridOnSurface(t)

	overlayManager.DrawGridPointer(
		manager.ModeGrid,
		image.Pt(120, 240),
		gridPointerAppearance,
	)
	overlayManager.Clear()
	surface.forget()

	overlayManager.UpdateGridMatches("a")

	if surface.paintedText(gridPointerAppearance.Char) {
		t.Errorf(
			"painted %v, want no pointer after the surface was cleared",
			surface.paintedStrings(),
		)
	}
}
