//go:build linux && cgo

package linux

import (
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

// closedSurface is an overlaySurface whose native handle is gone: alive says
// so, and every primitive fails the test. It is the state Destroy leaves a
// backend in, and the one each moved delegate used to guard for itself with an
// `o.raw != nil` prologue before those prologues became one shared alive()
// call.
type closedSurface struct{ t *testing.T }

func (s *closedSurface) alive() bool { return false }

func (s *closedSurface) touched(primitive string) {
	s.t.Helper()
	s.t.Errorf(
		"a delegate called %s on a backend whose native handle is closed; "+
			"that is a nil dereference inside cgo, not a no-op",
		primitive,
	)
}

func (s *closedSurface) surfaceScale() float64 { s.touched("surfaceScale"); return 1 }
func (s *closedSurface) ensureBuffers()        { s.touched("ensureBuffers") }
func (s *closedSurface) beginFrame() bool      { s.touched("beginFrame"); return false }
func (s *closedSurface) surfaceClear()         { s.touched("surfaceClear") }
func (s *closedSurface) clearFrame()           { s.touched("clearFrame") }
func (s *closedSurface) surfaceFlush()         { s.touched("surfaceFlush") }
func (s *closedSurface) surfaceHide()          { s.touched("surfaceHide") }
func (s *closedSurface) showIndicator()        { s.touched("showIndicator") }
func (s *closedSurface) finishIndicator()      { s.touched("finishIndicator") }
func (s *closedSurface) syncBeforeAnimation()  { s.touched("syncBeforeAnimation") }

func (s *closedSurface) surfaceClearRect(image.Rectangle) { s.touched("surfaceClearRect") }

func (s *closedSurface) rectPrim(image.Rectangle, uint32, uint32, float64) {
	s.touched("rectPrim")
}

func (s *closedSurface) roundedRectPrim(image.Rectangle, float64, uint32, uint32, float64) {
	s.touched("roundedRectPrim")
}

func (s *closedSurface) hintBadgePrim(
	image.Rectangle, float64, int, badge.HintArrow, uint32, uint32, float64,
) {
	s.touched("hintBadgePrim")
}

func (s *closedSurface) textPrim(_, _ string, _, _, _ float64, _ uint32) {
	s.touched("textPrim")
}

// movedDelegate is one of the sixteen exported methods the two backends used
// to declare identically and now share, called with arguments that would draw.
// Passing empty ones would let a delegate look guarded when it was only
// short-circuiting on its input.
type movedDelegate struct {
	name string
	call func(*sharedOverlay)
}

// drawMonitorSelectName is spelled once here because two other test files in
// this package already name the same method and goconst counts the literal.
const drawMonitorSelectName = "DrawMonitorSelect"

func movedDelegates() []movedDelegate {
	bounds := image.Rect(0, 0, 800, 600)
	dims := domain.GridDimensions{Rows: 2, Cols: 2}
	cell := domainGrid.NewGrid("abc", bounds, zap.NewNop()).AllCells()[0]

	return []movedDelegate{
		{"Hide", func(o *sharedOverlay) { o.Hide() }},
		{"Clear", func(o *sharedOverlay) { o.Clear() }},
		{"ClearRect", func(o *sharedOverlay) { o.ClearRect(image.Rect(1, 1, 20, 20)) }},
		{"UpdateGridMatches", func(o *sharedOverlay) { o.UpdateGridMatches("ab") }},
		{"ShowSubgrid", func(o *sharedOverlay) { o.ShowSubgrid(cell, gridcomponent.Style{}) }},
		{"SetHideUnmatched", func(o *sharedOverlay) { o.SetHideUnmatched(true) }},
		{"DrawGrid", func(o *sharedOverlay) {
			o.DrawGrid(domainGrid.NewGrid("abc", bounds, zap.NewNop()), "a", gridcomponent.Style{})
		}},
		{"DrawRecursiveGridWithSubKeyPreview", func(o *sharedOverlay) {
			o.DrawRecursiveGridWithSubKeyPreview(
				bounds, 0, "ab", dims, "cd", dims,
				recursivegridcomponent.Style{},
				recursivegridcomponent.VirtualPointerState{},
				false, 50,
			)
		}},
		{"DrawBadge", func(o *sharedOverlay) {
			o.DrawBadge(10, 10, "HINTS", overlayColors{}, overlayBadgeStyle{fontSize: 14})
		}},
		{"Flush", func(o *sharedOverlay) { o.Flush() }},
		{drawMonitorSelectName, func(o *sharedOverlay) {
			o.DrawMonitorSelect(
				[]manager.MonitorSelectTarget{{Label: "A", Bounds: bounds}},
				manager.MonitorSelectStyle{},
			)
		}},
		{"DrawHints", func(o *sharedOverlay) {
			o.DrawHints(
				[]*hintscomponent.Hint{
					hintscomponent.NewHint("aa", image.Pt(10, 10), image.Pt(20, 10), ""),
				},
				hintscomponent.StyleMode{},
				badge.HintOnTarget,
			)
		}},
		{"DrawMouseActionIndicator", func(o *sharedOverlay) {
			o.DrawMouseActionIndicator(image.Pt(10, 10), ports.MouseActionIndicatorStyle{})
		}},
		{"DrawHintSearchInput", func(o *sharedOverlay) {
			o.DrawHintSearchInput(
				"/ sav  3",
				hintscomponent.NewSearchInputFrame(image.Pt(10, 20), 300),
				hintscomponent.SearchInputStyle{},
			)
		}},
		{"HideHintSearchInput", func(o *sharedOverlay) {
			o.HideHintSearchInput(image.Rect(10, 20, 310, 60))
		}},
		{"setOriginOffset", func(o *sharedOverlay) { o.setOriginOffset(image.Pt(1920, 0)) }},
	}
}

// The sixteen delegates on sharedOverlay (fourteen moved there in #1415) are promoted
// into both backends, so the per-backend `o != nil && o.raw != nil` prologue
// they each carried is gone. The receiver half of it is the manager's job —
// promotion panics before any guard could run, which is why manager.go's
// nil-checked dispatch stays (ADR 0010). The handle half is this: a backend
// that was torn down, or one whose surface was never wired, must still return
// without reaching C.
//
// These are the states the manager can hold one in. The zero value is not
// hypothetical — it is what every test that stands a Manager up without a
// display server attaches, and what a backend looks like between construction
// and its first draw.
func TestSharedOverlay_MovedDelegates_ReachNoSurfaceWithoutALiveHandle(t *testing.T) {
	t.Parallel()

	states := map[string]func(*testing.T) *sharedOverlay{
		"no surface wired": func(*testing.T) *sharedOverlay {
			return &sharedOverlay{}
		},
		"handle closed": func(t *testing.T) *sharedOverlay {
			t.Helper()

			return &sharedOverlay{srf: &closedSurface{t: t}}
		},
		"x11 handle closed": func(*testing.T) *sharedOverlay {
			overlay := &x11Overlay{}
			overlay.srf = overlay

			return &overlay.sharedOverlay
		},
		"wlroots handle closed": func(*testing.T) *sharedOverlay {
			overlay := &wlrootsOverlay{}
			overlay.srf = overlay

			return &overlay.sharedOverlay
		},
	}

	for stateName, newState := range states {
		t.Run(stateName, func(t *testing.T) {
			t.Parallel()

			for _, delegate := range movedDelegates() {
				t.Run(delegate.name, func(t *testing.T) {
					t.Parallel()

					defer func() {
						if recovered := recover(); recovered != nil {
							t.Fatalf("%s panicked with no live handle: %v",
								delegate.name, recovered)
						}
					}()

					delegate.call(newState(t))
				})
			}
		})
	}
}

// The other half of the contract, and the reason manager.go nil-checks a
// backend pointer at every dispatch site: a moved delegate reached through a
// *nil* one
// panics on the promotion, before the guard above can run. That is what makes
// those checks load-bearing rather than an interface nobody got round to
// writing (ADR 0010), and it is the one premise this whole move rests on — so
// it is asserted rather than assumed.
//
// The methods that stayed per-backend are the control: they guard their own
// receiver and survive, which is exactly what the moved ones no longer do.
func TestBackends_APromotedDelegateOnANilBackendPanics(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		call      func()
		wantPanic bool
	}{
		"promoted Hide": {
			call:      func() { var overlay *x11Overlay; overlay.Hide() },
			wantPanic: true,
		},
		"promoted Flush": {
			call:      func() { var overlay *wlrootsOverlay; overlay.Flush() },
			wantPanic: true,
		},
		"promoted setOriginOffset": {
			call:      func() { var overlay *x11Overlay; overlay.setOriginOffset(image.Pt(1, 1)) },
			wantPanic: true,
		},
		"per-backend Show": {
			call: func() { var overlay *x11Overlay; overlay.Show() },
		},
		"per-backend Destroy": {
			call: func() { var overlay *wlrootsOverlay; overlay.Destroy() },
		},
		"per-backend Healthy": {
			call: func() { var overlay *x11Overlay; _ = overlay.Healthy() },
		},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			panicked := false

			func() {
				defer func() { panicked = recover() != nil }()

				testCase.call()
			}()

			if panicked != testCase.wantPanic {
				t.Errorf(
					"calling %s on a nil backend panicked = %t, want %t; "+
						"manager.go's nil-checked dispatch is sized to this answer",
					name, panicked, testCase.wantPanic,
				)
			}
		})
	}
}

// The two delegates that touch no surface keep working after the handle is
// gone, which is what their removed guards did: neither checked o.raw. They
// record what the next draw needs, and a draw that happens after a new handle
// is attached would otherwise draw on the wrong monitor, or stop hiding
// unmatched cells, for no reason either caller could see.
func TestSharedOverlay_StateOnlyDelegates_StillRecordWithoutALiveHandle(t *testing.T) {
	t.Parallel()

	overlay := &sharedOverlay{srf: &closedSurface{t: t}}

	overlay.SetHideUnmatched(true)
	overlay.setOriginOffset(image.Pt(1920, 0))

	if !overlay.hideUnmatched {
		t.Error("SetHideUnmatched did not record; it guards a surface it never touches")
	}

	if want := image.Pt(1920, 0); overlay.originOffset != want {
		t.Errorf("originOffset = %v, want %v", overlay.originOffset, want)
	}
}
