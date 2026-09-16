package modes

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	componentbisect "github.com/y3owk1n/neru/internal/app/components/bisect"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/bisect"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// bisectFixture is a handler in bisect mode over a 1000x800 screen that
// starts at (0,0), recording every cursor move.
type bisectFixture struct {
	handler *Handler
	moves   []image.Point
}

func newBisectFixture(t *testing.T) *bisectFixture {
	t.Helper()

	fixture := &bisectFixture{}
	appState := state.NewAppState()
	appState.SetMode(domain.ModeBisect)

	fixture.handler = newHandlerWithState(handlerState{
		config: &config.Config{
			Bisect: config.BisectConfig{
				Enabled: true,
				Hotkeys: config.DefaultConfig().Bisect.Hotkeys,
			},
		},
		appState: appState,
		logger:   zap.NewNop(),
		actionService: services.NewActionService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockOverlayPort{},
			&portmocks.MockSystemPort{
				MoveCursorToPointFunc: func(_ context.Context, point image.Point, _ bool) error {
					fixture.moves = append(fixture.moves, point)

					return nil
				},
			},
			zap.NewNop(),
		),
		bisect:       &components.BisectComponent{Context: &componentbisect.Context{}},
		screenBounds: image.Rect(0, 0, 1000, 800),
	})
	fixture.handler.modes = map[domain.Mode]Mode{
		domain.ModeBisect: NewBisectMode(&fixture.handler.handlerState),
	}

	fixture.handler.initializeBisectRegion(image.Rect(0, 0, 1000, 800))
	// Activation settles this from the flag; the fixture starts in follow.
	fixture.handler.bisect.Context.SetCursorFollowSelection(true)

	return fixture
}

func (f *bisectFixture) lastMove(t *testing.T) image.Point {
	t.Helper()

	if len(f.moves) == 0 {
		t.Fatal("the cursor never moved")
	}

	return f.moves[len(f.moves)-1]
}

func TestBisectCurrentMode_KeepsTheHalfAndMovesToItsCentre(t *testing.T) {
	fixture := newBisectFixture(t)

	fixture.handler.BisectCurrentMode(bisect.CutRight)

	if got := fixture.lastMove(t); got != image.Pt(750, 400) {
		t.Fatalf("cut right moved the cursor to %v, want the center of the right half", got)
	}

	fixture.handler.BisectCurrentMode(bisect.CutDownLeft)

	if got := fixture.lastMove(t); got != image.Pt(625, 600) {
		t.Fatalf("cut down-left moved the cursor to %v, want (625,600)", got)
	}
}

func TestBackspaceCurrentMode_TakesTheLastCutBack(t *testing.T) {
	fixture := newBisectFixture(t)

	fixture.handler.BisectCurrentMode(bisect.CutRight)
	fixture.handler.BisectCurrentMode(bisect.CutUp)
	fixture.handler.BackspaceCurrentMode()

	if got := fixture.lastMove(t); got != image.Pt(750, 400) {
		t.Fatalf("backspace moved the cursor to %v, want the right half's center", got)
	}
}

func TestResetCurrentMode_ReturnsToTheWholeRegion(t *testing.T) {
	fixture := newBisectFixture(t)

	fixture.handler.BisectCurrentMode(bisect.CutRight)
	fixture.handler.ResetCurrentMode()

	if got := fixture.lastMove(t); got != image.Pt(500, 400) {
		t.Fatalf("reset moved the cursor to %v, want the screen center", got)
	}

	if fixture.handler.bisect.Region.Depth() != 0 {
		t.Fatal("reset left history behind")
	}
}

func TestBisectFrame_LabelsTheQuadrantsWithTheBoundKeys(t *testing.T) {
	fixture := newBisectFixture(t)

	frame := fixture.handler.bisectFrame()

	if frame.Mode() != domain.ModeBisect || !frame.Bounds.Eq(image.Rect(0, 0, 1000, 800)) {
		t.Fatalf("frame = %+v, want the whole screen in bisect mode", frame)
	}

	if frame.Keys != "yubn" {
		t.Fatalf("quadrant keys = %q, want the default y u b n", frame.Keys)
	}
}

func TestBisectQuadrantKeys_LeavesAnUnboundQuadrantBlank(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Bisect.Hotkeys = map[string]config.StringOrStringArray{
		"q":    {config.CmdBisectUpLeft},
		"Left": {config.CmdBisectDownRight}, // a named key has no place in a cell
		"z":    {config.CmdBisectDownRight},
		"a":    {config.CmdBisectDownRight}, // ties go to the alphabetically first
	}

	bindings := cfg.ResolveKeymap(config.ModeNameBisect, "").Bindings()

	if got := bisectQuadrantKeys(bindings); got != "q  a" {
		t.Fatalf("quadrant keys = %q, want %q", got, "q  a")
	}
}

func TestBisectFrame_LabelsFollowThePerAppOverride(t *testing.T) {
	fixture := newBisectFixture(t)
	fixture.handler.config.Bisect.AppConfigs = []config.AppConfig{{
		BundleID: "com.example.Remapped",
		Hotkeys: map[string]config.StringOrStringArray{
			"y": {config.CmdBisectDownRight},
			"n": {config.CmdBisectUpLeft},
		},
	}}
	fixture.handler.focusedApp.publish("com.example.Remapped")

	if got := fixture.handler.bisectFrame().Keys; got != "nuby" {
		t.Fatalf("quadrant keys with the override = %q, want %q", got, "nuby")
	}
}

func TestBisectCurrentMode_HoldKeepsTheCursorAndDrawsThePointer(t *testing.T) {
	fixture := newBisectFixture(t)
	fixture.handler.bisect.Context.SetCursorFollowSelection(false)

	fixture.handler.BisectCurrentMode(bisect.CutRight)

	if len(fixture.moves) != 0 {
		t.Fatalf("hold mode moved the cursor to %v", fixture.moves)
	}

	frame := fixture.handler.bisectFrame()
	if !frame.Pointer.Visible || frame.Pointer.Position != image.Pt(750, 400) {
		t.Fatalf("frame pointer = %+v, want visible at the right half's center", frame.Pointer)
	}

	// Turning following on brings the real cursor onto the selection, and
	// the pointer stand-in comes off the frame.
	follow := true
	if enabled, ok := fixture.handler.SetCursorFollowSelection(follow); !ok || !enabled {
		t.Fatalf("SetCursorFollowSelection answered %t, %t", enabled, ok)
	}

	if got := fixture.lastMove(t); got != image.Pt(750, 400) {
		t.Fatalf("following on moved the cursor to %v, want the selection", got)
	}

	if fixture.handler.bisectFrame().Pointer.Visible {
		t.Fatal("pointer stand-in still drawn while the cursor follows")
	}
}

func TestBisectCurrentMode_DoesNothingOutsideBisectMode(t *testing.T) {
	fixture := newBisectFixture(t)
	fixture.handler.appState.SetMode(domain.ModeScroll)
	fixture.handler.modes[domain.ModeScroll] = NewScrollMode(&fixture.handler.handlerState)

	fixture.handler.BisectCurrentMode(bisect.CutRight)

	if len(fixture.moves) != 0 {
		t.Fatalf("bisect outside its mode moved the cursor to %v", fixture.moves)
	}
}

func TestCleanupBisectMode_DropsTheSession(t *testing.T) {
	fixture := newBisectFixture(t)
	fixture.handler.BisectCurrentMode(bisect.CutRight)

	fixture.handler.cleanupBisectMode()

	if fixture.handler.bisect.Region != nil {
		t.Fatal("cleanup left the region behind")
	}

	if _, ok := fixture.handler.bisect.Context.SelectionPoint(); ok {
		t.Fatal("cleanup left a selection point behind")
	}
}
