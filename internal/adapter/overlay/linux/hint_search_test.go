//go:build linux && cgo

package linux

import (
	"image"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/config"
)

// recordingManager stands a Manager up over a live recording surface, in the
// X11 shape — the two backends share every drawing method, so which one carries
// the surface changes nothing about what is painted.
func recordingManager() (*Manager, *recordingSurface) {
	surface := &recordingSurface{scale: 1}
	overlay := &x11Overlay{}
	overlay.srf = surface

	return &Manager{backend: linuxOverlayBackendX11, x11: overlay}, surface
}

// fixedTheme is a ThemeProvider that answers one mode.
type fixedTheme bool

func (t fixedTheme) IsDarkMode() bool { return bool(t) }

// searchStyle resolves a hint-search style the way the overlay's StyleResolver
// does, so the colors a theme picks are the ones these tests read back.
func searchStyle(dark bool) hints.SearchInputStyle {
	cfg := config.DefaultConfig()
	cfg.Hints.SearchInputUI.BackgroundColor = config.Color{Light: "#112233", Dark: "#445566"}
	cfg.Hints.SearchInputUI.TextColor = config.Color{Light: "#010203", Dark: "#040506"}
	cfg.Hints.SearchInputUI.BorderColor = config.Color{Light: "#0A0B0C", Dark: "#0D0E0F"}

	return hints.BuildSearchInputStyle(cfg.Hints, fixedTheme(dark))
}

// TestLinuxOverlayManager_DrawHintSearchInput_PaintsTheQueryOnTheActiveScreen
// pins what the badge is: a display of the query the mode handler already
// holds, painted where the overlay placed it and translated onto the monitor
// the hints are on. Nothing here reads a key — the query arrives as an
// argument, the same one macOS and Windows are handed.
func TestLinuxOverlayManager_DrawHintSearchInput_PaintsTheQueryOnTheActiveScreen(t *testing.T) {
	t.Parallel()

	manager, surface := recordingManager()
	manager.SetActiveScreenOrigin(image.Pt(1920, 0))

	err := manager.DrawHintSearchInput(
		"sav", 3,
		hints.NewSearchInputFrame(image.Pt(10, 20), 300),
		searchStyle(false),
	)
	if err != nil {
		t.Fatalf("DrawHintSearchInput() error = %v", err)
	}

	if len(surface.rects) != 1 {
		t.Fatalf("painted %d shapes, want exactly the badge", len(surface.rects))
	}

	painted := surface.rects[0]
	if painted.bounds.Min != image.Pt(1930, 20) {
		t.Errorf(
			"badge origin = %v, want %v: the frame is screen-local and belongs on the active monitor",
			painted.bounds.Min,
			image.Pt(1930, 20),
		)
	}

	if painted.bounds.Dx() != 300 {
		t.Errorf("badge width = %d, want the configured 300", painted.bounds.Dx())
	}

	if !painted.rounded {
		t.Error("the badge was drawn with the sharp-cornered primitive, " +
			"so its configured radius reaches nothing")
	}

	if painted.fill != 0xFF112233 {
		t.Errorf("badge fill = %#08x, want the light background the style carries", painted.fill)
	}

	if !surface.paintedText("/ sav  3") {
		t.Errorf("painted %v, want the query and its match count", surface.paintedStrings())
	}

	if surface.clears != 0 {
		t.Error("the badge cleared the surface; it is painted over the hints, not instead of them")
	}

	if surface.flushes == 0 {
		t.Error("the badge was never flushed, so it reaches the display on somebody else's tick")
	}
}

// TestLinuxOverlayManager_DrawHintSearchInput_PaintsTheThemeItsStyleCarries
// pins the second half of "the same style resolution as every other badge": the
// resolved Style is what reaches the surface, so a dark theme paints the dark
// colors without this backend resolving anything itself.
func TestLinuxOverlayManager_DrawHintSearchInput_PaintsTheThemeItsStyleCarries(t *testing.T) {
	t.Parallel()

	manager, surface := recordingManager()

	err := manager.DrawHintSearchInput(
		"s", 1,
		hints.NewSearchInputFrame(image.Pt(0, 0), 100),
		searchStyle(true),
	)
	if err != nil {
		t.Fatalf("DrawHintSearchInput() error = %v", err)
	}

	if len(surface.rects) != 1 {
		t.Fatalf("painted %d shapes, want exactly the badge", len(surface.rects))
	}

	if got := surface.rects[0].fill; got != 0xFF445566 {
		t.Errorf("badge fill = %#08x, want the dark background the style carries", got)
	}

	if got := surface.rects[0].border; got != 0xFF0D0E0F {
		t.Errorf("badge border = %#08x, want the dark border the style carries", got)
	}
}

// TestLinuxOverlayManager_HideHintSearchInput_TakesTheBadgeOffTheSurface pins
// the other direction. A Linux badge is painted onto the one shared surface
// rather than into a window of its own, so hiding it means putting back what it
// covered: the hints are repainted where there are hints, and the rectangle is
// erased where there are none. Erasing unconditionally would punch a hole
// through the labels the user is about to type.
func TestLinuxOverlayManager_HideHintSearchInput_TakesTheBadgeOffTheSurface(t *testing.T) {
	t.Parallel()

	t.Run("with hints on the surface", func(t *testing.T) {
		t.Parallel()

		manager, surface := recordingManager()

		drawErr := manager.DrawHintsWithStyle(
			[]*hints.Hint{hints.NewHint("aa", image.Pt(100, 100), image.Pt(20, 10), "")},
			hints.StyleMode{},
		)
		if drawErr != nil {
			t.Fatalf("DrawHintsWithStyle() error = %v", drawErr)
		}

		searchErr := manager.DrawHintSearchInput(
			"sav", 3,
			hints.NewSearchInputFrame(image.Pt(10, 20), 300),
			searchStyle(false),
		)
		if searchErr != nil {
			t.Fatalf("DrawHintSearchInput() error = %v", searchErr)
		}

		before := surface.clears
		surface.texts = nil

		manager.HideHintSearchInput()

		if surface.clears == before {
			t.Error("hiding the badge left it on the surface; the hints were never repainted")
		}

		if surface.paintedText("/ sav  3") {
			t.Error("the repaint put the badge back on the surface")
		}

		if !surface.paintedText("aa") {
			t.Error("the repaint dropped the hint label the badge was painted over")
		}
	})

	t.Run("with nothing under it", func(t *testing.T) {
		t.Parallel()

		manager, surface := recordingManager()

		searchErr := manager.DrawHintSearchInput(
			"sav", 3,
			hints.NewSearchInputFrame(image.Pt(10, 20), 300),
			searchStyle(false),
		)
		if searchErr != nil {
			t.Fatalf("DrawHintSearchInput() error = %v", searchErr)
		}

		painted := surface.rects[0].bounds

		manager.HideHintSearchInput()

		if len(surface.clearedRects) != 1 || surface.clearedRects[0] != painted {
			t.Errorf("erased %v, want exactly the badge rectangle %v",
				surface.clearedRects, painted)
		}
	})

	t.Run("with no badge drawn", func(t *testing.T) {
		t.Parallel()

		manager, surface := recordingManager()

		manager.HideHintSearchInput()

		if len(surface.clearedRects) != 0 || surface.clears != 0 {
			t.Error("hiding a badge that was never drawn touched the surface")
		}
	})
}

// silentSurface is a live overlaySurface that records nothing. The race test
// below uses it rather than recordingSurface deliberately: a recorder needs a
// lock of its own to be driven from two goroutines, and that lock would order
// every draw against every other one — hiding exactly the unsynchronized access
// the detector is there to find.
type silentSurface struct{}

func (s *silentSurface) alive() bool                      { return true }
func (s *silentSurface) surfaceScale() float64            { return 1 }
func (s *silentSurface) ensureBuffers()                   {}
func (s *silentSurface) beginFrame() bool                 { return true }
func (s *silentSurface) surfaceClear()                    {}
func (s *silentSurface) clearFrame()                      {}
func (s *silentSurface) surfaceClearRect(image.Rectangle) {}
func (s *silentSurface) surfaceFlush()                    {}
func (s *silentSurface) surfaceHide()                     {}
func (s *silentSurface) showIndicator()                   {}
func (s *silentSurface) finishIndicator()                 {}
func (s *silentSurface) syncBeforeAnimation()             {}

func (s *silentSurface) rectPrim(image.Rectangle, uint32, uint32, float64) {}

func (s *silentSurface) roundedRectPrim(image.Rectangle, float64, uint32, uint32, float64) {}

func (s *silentSurface) hintBadgePrim(
	image.Rectangle, float64, int, badge.HintArrow, uint32, uint32, float64,
) {
}

func (s *silentSurface) textPrim(_, _ string, _, _, _ float64, _ uint32) {}

// TestLinuxOverlayManager_HintSearchInputIsSerializedWithTheHintsDraw is the
// -race regression for the screen state this feature added. searchBadgeRect on
// the manager and the lastHints trio on the surface are written by the badge
// draw, by the hints draw that erases it and by the hide that puts it back,
// which reach the manager from the mode handler and from an indicator tick
// alike — so all three have to stay inside renderMu. It only runs on the Linux
// leg, `go test -race ./internal/adapter/overlay/linux/` there.
func TestLinuxOverlayManager_HintSearchInputIsSerializedWithTheHintsDraw(t *testing.T) {
	t.Parallel()

	overlay := &x11Overlay{}
	overlay.srf = &silentSurface{}
	manager := &Manager{backend: linuxOverlayBackendX11, x11: overlay}

	drawn := []*hints.Hint{hints.NewHint("aa", image.Pt(100, 100), image.Pt(20, 10), "")}
	frame := hints.NewSearchInputFrame(image.Pt(10, 20), 300)
	style := searchStyle(false)

	// Enough rounds that the two loops genuinely overlap: at 200 an unlocked
	// write escaped the detector on a run more often than not.
	const rounds = 5000

	var (
		group    sync.WaitGroup
		refusals atomic.Int64
	)

	group.Add(2)

	go func() {
		defer group.Done()

		for range rounds {
			if manager.DrawHintsWithStyle(drawn, hints.StyleMode{}) != nil {
				refusals.Add(1)
			}
		}
	}()

	go func() {
		defer group.Done()

		for range rounds {
			if manager.DrawHintSearchInput("sav", 3, frame, style) != nil {
				refusals.Add(1)
			}

			manager.HideHintSearchInput()
		}
	}()

	group.Wait()

	if got := refusals.Load(); got != 0 {
		t.Errorf("%d of %d draws were refused against a live surface", got, rounds*2)
	}

	// Both loops end on a call that forgets the badge — the hide, and the hints
	// draw that erases it — so whichever landed last, nothing is recorded as
	// being on screen. A non-empty rectangle here is a write that escaped the
	// lock, not a timing difference.
	manager.renderMu.Lock()
	defer manager.renderMu.Unlock()

	if !manager.searchBadgeRect.Empty() {
		t.Errorf("searchBadgeRect = %v after every badge was hidden, want the empty rectangle",
			manager.searchBadgeRect)
	}
}
