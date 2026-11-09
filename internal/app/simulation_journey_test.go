package app_test

// Full-loop user-journey tests. Each test drives the real app with nothing
// but key strings, the same input the native event tap delivers, and
// asserts on the observable outcome: what was drawn, where the cursor went,
// and what was clicked or scrolled. See simulation_harness_test.go.

import (
	"context"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/heldmotion"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	hintsHotkey  = "Primary+Shift+Space"
	gridHotkey   = "Primary+Shift+G"
	scrollHotkey = "Primary+Shift+S"
)

// Steps more than one journey binds.
const (
	stepScroll            = "scroll"
	stepGrid              = "grid"
	stepHintsRepeatClicks = "hints left_click --repeat"
)

// The two fixture applications a journey switches between. The desktop starts
// focused on the first, and the second is what it can switch away to.
const (
	simFixtureAppName   = "Sim Fixture"
	simOtherAppBundleID = "com.example.otherapp"
	simOtherAppName     = "Other App"
)

// threeButtons is a fixture of three well-separated clickable elements.
func threeButtons(t *testing.T) []*element.Element {
	t.Helper()

	return []*element.Element{
		simElement(t, "save", image.Rect(100, 100, 220, 140), "Save"),
		simElement(t, "cancel", image.Rect(300, 100, 420, 140), "Cancel"),
		simElement(t, "search", image.Rect(100, 300, 500, 340), "Search"),
	}
}

// TestSimulation_HintsJourney_SelectMovesCursor covers the default journey:
// activation hotkey -> hints drawn for every element -> typing a label moves
// the cursor to that element's center and re-activates hints (no pending
// action was requested, so nothing is clicked).
func TestSimulation_HintsJourney_SelectMovesCursor(t *testing.T) {
	elements := threeButtons(t)
	sim := newSimHarness(t, simConfig(), elements)

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	labels := sim.overlay.lastHintLabels()
	if len(labels) != len(elements) {
		t.Fatalf("expected %d hint labels, got %d (%v)", len(elements), len(labels), labels)
	}

	seen := map[string]bool{}
	for _, l := range labels {
		if l == "" || seen[l] {
			t.Fatalf("hint labels must be unique and non-empty, got %v", labels)
		}

		seen[l] = true
	}

	drawsBefore := sim.overlay.hintDrawCount()
	movesBefore := sim.cursor.moveCount()

	sim.typeLabel(labels[0])

	sim.waitFor("cursor moved to a hinted element", func() bool {
		return sim.cursor.moveCount() > movesBefore
	})

	target := sim.cursor.position()

	centers := make([]image.Point, len(elements))
	found := false

	for i, e := range elements {
		centers[i] = e.Center()
		if target == e.Center() {
			found = true
		}
	}

	if !found {
		t.Fatalf("cursor landed at %v, expected one of the element centers %v", target, centers)
	}

	// No pending action: selection must not click anything...
	if clicks := sim.ax.recordedClicks(); len(clicks) != 0 {
		t.Fatalf("expected no clicks for actionless hint selection, got %v", clicks)
	}

	// ...and hints re-activate for the next selection.
	sim.waitFor("hints re-drawn after selection", func() bool {
		return sim.overlay.hintDrawCount() > drawsBefore
	})
}

// TestSimulation_HintsJourney_ClickAction covers the canonical "click a
// button without a mouse" journey. A binding with an explicit action
// ("hints left_click") makes typing the label move the cursor to the element
// center, click there, and drop back to idle.
//
// The three spellings of that binding run the same journey. A binding is text
// the daemon reads for itself, so this is where a flag that parses but never
// reaches the mode would show up, as a hint that highlights and then does
// nothing.
func TestSimulation_HintsJourney_ClickAction(t *testing.T) {
	bindings := map[string]string{
		"positional action": "hints left_click",
		"long flag":         "hints --action=left_click",
		"short flag":        "hints -a left_click",
	}

	for name, binding := range bindings {
		t.Run(name, func(t *testing.T) {
			cfg := simConfig()
			cfg.Hotkeys.Bindings[hintsHotkey] = []string{binding}

			save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
			sim := newSimHarness(t, cfg, []*element.Element{save})

			sim.pressHotkey(hintsHotkey)
			sim.waitMode(domain.ModeHints)
			sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

			labels := sim.overlay.lastHintLabels()
			if len(labels) != 1 {
				t.Fatalf("expected exactly one hint label, got %v", labels)
			}

			sim.typeLabel(labels[0])

			sim.waitFor("click recorded", func() bool { return len(sim.ax.recordedClicks()) > 0 })

			clicks := sim.ax.recordedClicks()
			if len(clicks) != 1 {
				t.Fatalf("expected exactly one click, got %d", len(clicks))
			}

			if clicks[0].point != save.Center() {
				t.Fatalf("click landed at %v, expected element center %v",
					clicks[0].point, save.Center())
			}

			if got := sim.cursor.position(); got != save.Center() {
				t.Fatalf("cursor at %v after click, expected element center %v",
					got, save.Center())
			}

			sim.waitMode(domain.ModeIdle)
		})
	}
}

// TestSimulation_HintsEscape covers bailing out. Escape exits hints, hides
// the overlay, and returns to idle without side effects.
func TestSimulation_HintsEscape(t *testing.T) {
	sim := newSimHarness(t, simConfig(), threeButtons(t))

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("overlay visible", sim.overlay.isVisible)

	sim.press("Escape")

	sim.waitMode(domain.ModeIdle)
	sim.waitFor("overlay hidden", func() bool { return !sim.overlay.isVisible() })

	if clicks := sim.ax.recordedClicks(); len(clicks) != 0 {
		t.Fatalf("escape must not click, got %v", clicks)
	}
}

// TestSimulation_GridJourney covers the accessibility-independent path. Grid
// mode needs no elements at all. Typing a cell's coordinate moves the cursor
// into that cell.
func TestSimulation_GridJourney(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	grid := sim.overlay.lastGrid()

	cells := grid.Cells()
	if len(cells) == 0 {
		t.Fatal("grid drawn with zero cells")
	}

	// Pick a cell away from index 0 so a "cursor never moved" bug cannot pass.
	cell := cells[len(cells)/2]

	movesBefore := sim.cursor.moveCount()
	sim.typeLabel(cell.Coordinate())

	sim.waitFor("cursor moved into selected cell", func() bool {
		return sim.cursor.moveCount() > movesBefore
	})

	if pos := sim.cursor.position(); !pos.In(cell.Bounds()) {
		t.Fatalf(
			"cursor at %v, expected inside cell %q bounds %v",
			pos, cell.Coordinate(), cell.Bounds(),
		)
	}

	// A complete label opens the finer grid inside the cell, which is the
	// other thing a user sees for that last keystroke.
	if got := sim.overlay.subgridCount(); got != 1 {
		t.Errorf("subgrids opened = %d, want 1 after a full label was typed", got)
	}
}

// TestSimulation_GridNarrowingCostsNoRedraw is the promise ADR 0003 makes a
// user. Typing in grid mode narrows what is on screen without redrawing the
// grid or putting the overlay up again. It asserts the domain value the user's
// keystroke produced, the prefix the cells are matched against, rather than
// which call carried it.
func TestSimulation_GridNarrowingCostsNoRedraw(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	cells := sim.overlay.lastGrid().Cells()
	if len(cells) == 0 {
		t.Fatal("grid drawn with zero cells")
	}

	label := cells[len(cells)/2].Coordinate()
	if len([]rune(label)) < 2 {
		t.Fatalf("grid label %q is one character; it cannot be narrowed", label)
	}

	drawsBefore := sim.overlay.gridDrawCount()
	showsBefore := sim.overlay.showCount()

	// Labels are drawn upper case and the grid matches on them. A user types
	// the key, so the two differ in case and only in case.
	first := string([]rune(label)[0])
	sim.press(strings.ToLower(first))

	sim.waitFor("grid narrowed to what was typed", func() bool {
		prefix, narrowed := sim.overlay.lastMatchPrefix()

		return narrowed && prefix == first
	})

	if got := sim.overlay.gridDrawCount(); got != drawsBefore {
		t.Errorf("grid draws = %d after one keystroke, want %d: narrowing redrew the grid",
			got, drawsBefore)
	}

	if got := sim.overlay.showCount(); got != showsBefore {
		t.Errorf("overlay shows = %d after one keystroke, want %d: narrowing repeated the "+
			"window sequence", got, showsBefore)
	}

	if hidden, asked := sim.overlay.lastHideUnmatched(); !asked || !hidden {
		t.Errorf("unmatched cells asked to hide = %v (asked = %v), want true: the cells that "+
			"no longer match are still on screen", hidden, asked)
	}

	if !sim.overlay.isVisible() {
		t.Error("the grid left the screen while the user was typing")
	}
}

// stopCursorFollowing turns cursor-follow-selection off with the key bound to
// it and waits for the change to land. The toggle is a mode hotkey, so the app
// runs it on a goroutine. The pointer refresh it owes is what says it ran.
func stopCursorFollowing(sim *simHarness) {
	sim.t.Helper()

	before := sim.overlay.gridSurfaceUpdates()

	sim.press("`")

	sim.waitFor("the cursor stopped following the selection", func() bool {
		return sim.overlay.gridSurfaceUpdates() > before
	})
}

// TestSimulation_GridSelectionUpdatesTheSurfaceOnce is the other half of what
// ADR 0003 promises, and what #1492 was. The keystroke that picks a cell opens
// the subgrid *and* moves the selection, and where both are painted onto one
// surface saying it in two calls repaints that surface twice for one key. The
// mode says it once.
//
// Cursor-follow is turned off first ("`"), because that is the session this is
// about. With the real cursor following the selection there is no pointer
// stand-in to move and the second update never existed. The last keystroke of
// the label is the one measured. The ones before it narrow, which is the path
// the test above covers.
func TestSimulation_GridSelectionUpdatesTheSurfaceOnce(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	stopCursorFollowing(sim)

	cells := sim.overlay.lastGrid().Cells()
	if len(cells) == 0 {
		t.Fatal("grid drawn with zero cells")
	}

	label := []rune(cells[len(cells)/2].Coordinate())
	if len(label) < 2 {
		t.Fatalf("grid label %q is one character; it has no narrowing half", string(label))
	}

	sim.typeLabel(string(label[:len(label)-1]))

	updatesBefore := sim.overlay.gridSurfaceUpdates()
	subgridsBefore := sim.overlay.subgridCount()

	sim.press(strings.ToLower(string(label[len(label)-1])))

	sim.waitFor("the subgrid opened", func() bool {
		return sim.overlay.subgridCount() > subgridsBefore
	})

	if got := sim.overlay.gridSurfaceUpdates() - updatesBefore; got != 1 {
		t.Errorf("the selection keystroke asked the grid surface to change %d times, want 1",
			got)
	}

	// And the one call carried both. The pointer stands on the cell the subgrid
	// was opened inside, which is the only thing on screen saying where the
	// selection is for this user.
	pointer, drawn := sim.overlay.lastGridPointer()
	if !drawn || !pointer.Visible {
		t.Errorf("selection pointer = %+v (drawn = %v), want it on screen with the subgrid",
			pointer, drawn)
	}
}

// TestSimulation_GridNoSubgridJourney covers --no-subgrid. A full label
// completes the selection at the cell's center on its last key, so a pending
// action runs and the mode exits without a subgrid ever opening, and --repeat
// reopens grid still without one.
func TestSimulation_GridNoSubgridJourney(t *testing.T) {
	tests := []struct {
		name    string
		binding string
		rounds  int
		repeat  bool
	}{
		{name: "action exits", binding: "grid --no-subgrid --action left_click", rounds: 1},
		{
			name:    "repeat keeps the flag",
			binding: "grid --no-subgrid --action left_click --repeat",
			rounds:  2,
			repeat:  true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := simConfig()
			cfg.Hotkeys.Bindings[gridHotkey] = []string{testCase.binding}
			sim := newSimHarness(t, cfg, nil)

			sim.pressHotkey(gridHotkey)
			sim.waitMode(domain.ModeGrid)
			sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

			cells := sim.overlay.lastGrid().Cells()
			cell := cells[len(cells)/2]

			for round := 1; round <= testCase.rounds; round++ {
				drawsBefore := sim.overlay.gridDrawCount()
				sim.typeLabel(cell.Coordinate())

				sim.waitFor("click recorded", func() bool {
					return len(sim.ax.recordedClicks()) == round
				})

				if testCase.repeat {
					sim.waitFor("grid reopened by --repeat", func() bool {
						return sim.overlay.gridDrawCount() > drawsBefore
					})
				}
			}

			for _, click := range sim.ax.recordedClicks() {
				if click.point != cell.Center() {
					t.Errorf("click at %v, want the cell center %v", click.point, cell.Center())
				}
			}

			if got := sim.overlay.subgridCount(); got != 0 {
				t.Errorf("subgrids opened = %d, want 0 with --no-subgrid", got)
			}

			if !testCase.repeat {
				sim.waitMode(domain.ModeIdle)
			} else if mode := sim.app.CurrentMode(); mode != domain.ModeGrid {
				t.Errorf("mode = %v after a repeat click, want grid", mode)
			}
		})
	}
}

// TestSimulation_GridNoSubgridWithoutActionKeepsTheLabel covers --no-subgrid
// with no action. The full label selects the cell and the mode stays open with
// the label still typed, so backspace edits it rather than leaving a subgrid.
func TestSimulation_GridNoSubgridWithoutActionKeepsTheLabel(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[gridHotkey] = []string{"grid --no-subgrid"}
	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	cells := sim.overlay.lastGrid().Cells()
	cell := cells[len(cells)/2]
	label := strings.ToUpper(cell.Coordinate())

	sim.typeLabel(label)
	sim.waitFor("cell highlighted by the full label", func() bool {
		prefix, _ := sim.overlay.lastMatchPrefix()

		return prefix == label
	})

	// The cursor follows the selection by default, so its position is the
	// selection.
	if got := sim.cursor.position(); got != cell.Center() {
		t.Errorf("cursor = %v, want the cell center %v", got, cell.Center())
	}

	sim.press("Backspace")
	sim.waitFor("backspace edits the label", func() bool {
		prefix, _ := sim.overlay.lastMatchPrefix()

		return prefix == label[:len(label)-1]
	})

	if got := sim.overlay.subgridCount(); got != 0 {
		t.Errorf("subgrids opened = %d, want 0 with --no-subgrid", got)
	}

	if clicks := sim.ax.recordedClicks(); len(clicks) != 0 {
		t.Errorf("clicks = %v, want none without --action", clicks)
	}

	if mode := sim.app.CurrentMode(); mode != domain.ModeGrid {
		t.Errorf("mode = %v, want grid still open", mode)
	}
}

// moveCellKey is the key this file's arrow-key journey binds `move_cell` to.
// Grid mode's default hotkeys point the arrows at `move_mouse_*`, so sliding an
// open subgrid is a binding a user writes, and it reaches the same subgrid
// callback the selection keystroke does, which is why it is measured here.
const moveCellKey = "Right"

// TestSimulation_MovingAnOpenSubgridUpdatesTheSurfaceOncePerPress is the third
// keystroke #1492 names. A held arrow inside a subgrid re-opens the subgrid over
// the neighboring cell and moves the selection with it, once per repeat. Each
// press must cost the surface one change, or a held key multiplies the one the
// spec is about.
func TestSimulation_MovingAnOpenSubgridUpdatesTheSurfaceOncePerPress(t *testing.T) {
	cfg := simConfig()
	cfg.Grid.Hotkeys[moveCellKey] = config.StringOrStringArray{
		"action move_cell --direction=right",
	}

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	stopCursorFollowing(sim)

	cells := sim.overlay.lastGrid().Cells()
	if len(cells) == 0 {
		t.Fatal("grid drawn with zero cells")
	}

	// The first cell, so there is a neighbor to the right to slide onto.
	sim.typeLabel(cells[0].Coordinate())

	sim.waitFor("the subgrid opened", func() bool { return sim.overlay.subgridCount() > 0 })

	const presses = 3

	for press := range presses {
		subgridsBefore := sim.overlay.subgridCount()
		updatesBefore := sim.overlay.gridSurfaceUpdates()

		sim.press(moveCellKey)

		sim.waitFor("the subgrid moved", func() bool {
			return sim.overlay.subgridCount() > subgridsBefore
		})

		if got := sim.overlay.gridSurfaceUpdates() - updatesBefore; got != 1 {
			t.Fatalf("press %d asked the grid surface to change %d times, want 1",
				press+1, got)
		}
	}
}

// TestSimulation_LeavingGridRepaintsNothingOnTheWayOut is the teardown half of
// the same promise (#1492). The grid is about to be taken off the screen, so
// resetting what the overlay still holds for it must not repaint it first. The
// overlay's own leaving half drops that state, and a mode that reset it from
// here repainted a grid twice to throw it away a moment later.
func TestSimulation_LeavingGridRepaintsNothingOnTheWayOut(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	// A visible pointer is the state teardown used to repaint for.
	stopCursorFollowing(sim)

	cells := sim.overlay.lastGrid().Cells()
	if len(cells) == 0 {
		t.Fatal("grid drawn with zero cells")
	}

	sim.typeLabel(cells[len(cells)/2].Coordinate())

	sim.waitFor("the selection is on screen", func() bool {
		pointer, drawn := sim.overlay.lastGridPointer()

		return drawn && pointer.Visible
	})

	updatesBefore := sim.overlay.gridSurfaceUpdates()

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
	sim.waitFor("grid off screen", func() bool { return !sim.overlay.isVisible() })

	if got := sim.overlay.gridSurfaceUpdates() - updatesBefore; got != 0 {
		t.Errorf("leaving grid mode asked the grid surface to change %d times, want 0: it was "+
			"repainted on the way to being cleared", got)
	}
}

// TestSimulation_GridLeavesNothingOnScreen pins the leaving half. Exiting grid
// mode takes the grid off the screen and returns the overlay to idle, so the
// next mode does not appear next to the last one.
func TestSimulation_GridLeavesNothingOnScreen(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid on screen", func() bool {
		return sim.overlay.lastGrid() != nil && sim.overlay.isVisible()
	})

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	sim.waitFor("grid off screen", func() bool {
		return !sim.overlay.isVisible() && len(sim.overlay.drawnModeNames()) == 0
	})
}

// TestSimulation_RecursiveGridZoomsWithoutShowingAgain covers the other grid
// surface. Every keystroke repaints it, and none of them puts the overlay up a
// second time.
func TestSimulation_RecursiveGridZoomsWithoutShowingAgain(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(recursiveGridHotkey)
	sim.waitMode(domain.ModeRecursiveGrid)
	sim.waitFor("recursive grid drawn", func() bool {
		_, ok := sim.overlay.lastRecursiveGridBounds()

		return ok
	})

	drawsBefore := sim.overlay.recursiveGridDrawCount()
	showsBefore := sim.overlay.showCount()

	sim.press("r")

	sim.waitFor("recursive grid redrawn for the keystroke", func() bool {
		return sim.overlay.recursiveGridDrawCount() > drawsBefore
	})

	if got := sim.overlay.showCount(); got != showsBefore {
		t.Errorf("overlay shows = %d after one keystroke, want %d: zooming repeated the "+
			"window sequence", got, showsBefore)
	}
}

// TestSimulation_ScrollJourney covers scroll mode, where j scrolls and Escape leaves
// the mode and lands back in idle.
func TestSimulation_ScrollJourney(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	sim.press("j")

	sim.waitFor("scroll recorded", func() bool { return len(sim.ax.recordedScrolls()) > 0 })

	scrolls := sim.ax.recordedScrolls()
	if scrolls[0].Y == 0 {
		t.Fatalf("expected vertical scroll delta for 'j', got %v", scrolls[0])
	}

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_ModeIndicatorDisappearsWhenModeExits pins what a user sees
// after leaving a mode. The indicator that named it is gone. Scroll mode is
// the one whose indicator is on by default.
//
// The indicator is driven by a polling goroutine racing mode teardown, which
// is exactly how one used to be left behind on screen, so this asserts the
// visibility the app last asked for rather than any single call.
func TestSimulation_ModeIndicatorDisappearsWhenModeExits(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	sim.waitFor("mode indicator shown", func() bool {
		visible, asked := sim.overlay.modeIndicatorVisibility()

		return asked && visible
	})

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	sim.waitFor("mode indicator hidden", func() bool {
		visible, asked := sim.overlay.modeIndicatorVisibility()

		return asked && !visible
	})

	// Nothing may put it back once the mode is gone. The polling goroutine is
	// stopped before the indicator is hidden, and a late tick would show it.
	time.Sleep(4 * indicatorSettleWindow)

	if visible, _ := sim.overlay.modeIndicatorVisibility(); visible {
		t.Fatal("mode indicator was shown again after the mode exited")
	}
}

// indicatorSettleWindow is one indicator poll interval, the window in which a
// late tick could redraw an indicator that has just been hidden.
const indicatorSettleWindow = 16 * time.Millisecond

// TestSimulation_ModeSwitch covers chaining modes in one session: hints ->
// escape -> grid, exercising mode teardown between activations.
func TestSimulation_ModeSwitch(t *testing.T) {
	sim := newSimHarness(t, simConfig(), threeButtons(t))

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_HintsWithoutElements covers the failure path a real user
// hits on an empty screen. Activation is abandoned, the app stays idle and
// keeps working (grid still activates afterwards).
func TestSimulation_HintsWithoutElements(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(hintsHotkey)
	sim.neverMode(domain.ModeHints, 250*time.Millisecond)

	if sim.app.CurrentMode() != domain.ModeIdle {
		t.Fatalf("expected idle after abandoned activation, got %v", sim.app.CurrentMode())
	}

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
}

// TestSimulation_ExcludedApp covers the exclusion list. When the focused app
// is excluded, no mode activates. The binder registers no hotkeys for it, so
// the activation comes the way an IPC command would send it.
func TestSimulation_ExcludedApp(t *testing.T) {
	cfg := simConfig()
	cfg.General.ExcludedApps = []string{simFixtureBundleID}

	sim := newSimHarness(t, cfg, threeButtons(t))

	sim.app.ActivateMode(domain.ModeHints)
	sim.neverMode(domain.ModeHints, 250*time.Millisecond)

	if sim.overlay.hintDrawCount() != 0 {
		t.Fatal("hints must not be drawn for an excluded app")
	}
}

// TestSimulation_ExcludedAppsFollowAReload covers a reload that changes
// excluded_apps. Every check that a mode may open has to use the reloaded
// list, in whatever letter case the file spells the app.
func TestSimulation_ExcludedAppsFollowAReload(t *testing.T) {
	reload := func(t *testing.T, sim *simHarness, excludedApps string) {
		t.Helper()

		configPath := filepath.Join(t.TempDir(), "config.toml")

		writeErr := os.WriteFile(configPath, fmt.Appendf(nil, `
[general]
excluded_apps = [%s]

[hotkeys]
%q = "hints"
`, excludedApps, hintsHotkey), 0o600)
		if writeErr != nil {
			t.Fatalf("failed to write the reloaded config: %v", writeErr)
		}

		reloadErr := sim.app.ReloadConfig(context.Background(), configPath)
		if reloadErr != nil {
			t.Fatalf("ReloadConfig() error = %v", reloadErr)
		}
	}

	t.Run("an app taken off the list opens hints again", func(t *testing.T) {
		cfg := simConfig()
		cfg.General.ExcludedApps = []string{simFixtureBundleID}

		sim := newSimHarness(t, cfg, threeButtons(t))

		reload(t, sim, "")

		sim.pressHotkey(hintsHotkey)
		sim.waitMode(domain.ModeHints)
	})

	t.Run("an app put on the list no longer opens a mode", func(t *testing.T) {
		sim := newSimHarness(t, simConfig(), threeButtons(t))

		reload(t, sim, fmt.Sprintf("%q", strings.ToUpper(simFixtureBundleID)))

		sim.app.ActivateMode(domain.ModeGrid)
		sim.neverMode(domain.ModeGrid, 250*time.Millisecond)
	})
}

const (
	recursiveGridHotkey = "Primary+Shift+C"
	bisectHotkey        = "Primary+Shift+X"
	bisectWindowHotkey  = "Primary+Shift+V"
	windowScopeHotkey   = "Primary+Shift+W"
)

// manyButtons builds a count-sized grid of well-separated clickable buttons,
// enough to force multi-character hint labels.
func manyButtons(tb testing.TB, count int) []*element.Element {
	tb.Helper()

	elements := make([]*element.Element, 0, count)
	for index := range count {
		col := index % 4
		row := index / 4
		bounds := image.Rect(100+col*200, 100+row*100, 220+col*200, 140+row*100)
		elements = append(
			elements,
			simElement(tb, fmt.Sprintf("btn-%d", index), bounds, fmt.Sprintf("Button %d", index)),
		)
	}

	return elements
}

// TestSimulation_HintsSearchJourney covers text search. Pressing "/" opens search,
// typing a query narrows the hints to matching elements, Return confirms,
// and selecting the surviving label lands on the matched element.
//
// The second case runs the identical journey on an overlay that reports
// CodeNotSupported for the search input, as the Linux overlay does where no backend
// draws that badge (#1328). The user sees no "/ sav 1 /" box and loses nothing
// else, because the query still reaches the hints through the event tap's key stream.
// A refusal that surfaced as a failed mode, or that stopped the keystrokes
// getting through, is what this second case pins.
//
// Each case also types a capital the way the Linux and Windows taps name it,
// "Shift+c". Dropped, it would leave "a", which matches every button.
func TestSimulation_HintsSearchJourney(t *testing.T) {
	overlays := map[string]func(testing.TB, *config.Config, []*element.Element) *simHarness{
		"search input drawn":   newSimHarness,
		"search input refused": newSimHarnessRefusingHintSearch,
	}

	queries := []struct {
		name   string
		keys   []string
		target int
	}{
		{"lowercase", []string{"s", "a", "v"}, 0},       // matches only "Save"
		{"shifted letter", []string{"Shift+c", "a"}, 1}, // matches only "Cancel"
	}

	for name, newHarness := range overlays {
		for _, query := range queries {
			t.Run(name+" "+query.name, func(t *testing.T) {
				elements := threeButtons(t)
				sim := newHarness(t, simConfig(), elements)

				sim.pressHotkey(hintsHotkey)
				sim.waitMode(domain.ModeHints)
				sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

				// "/" dispatches search_hints asynchronously. Wait for search to be
				// open before typing, or the query characters would be read as
				// labels. The wait is on the ask rather than on anything drawn,
				// because the refusing overlay draws nothing.
				sim.press("/")
				sim.waitFor("search input asked for", func() bool {
					return sim.overlay.searchInputAskCount() > 0
				})

				sim.press(query.keys...)

				sim.waitFor("hints narrowed to the query", func() bool {
					return len(sim.overlay.lastHintLabels()) == 1
				})

				// Return selects the one match itself, within the keystroke.
				sim.press("Return")

				target := elements[query.target].Center()
				if got := sim.cursor.position(); got != target {
					t.Fatalf(
						"cursor after Return = %v, want the searched element at %v",
						got,
						target,
					)
				}
			})
		}
	}
}

// TestSimulation_HintSearchFieldConfirmSelectsTheMatch covers Return in a
// platform search field, which confirms through its callback rather than the
// key stream. With --action, --repeat and --search, the selection clicks and
// reopens hints with a new field from inside the old field's callback. The
// click has to have happened by the time that callback returns.
func TestSimulation_HintSearchFieldConfirmSelectsTheMatch(t *testing.T) {
	const searchClickHotkey = "Primary+Shift+S"

	cfg := simConfig()
	cfg.Hotkeys.Bindings[searchClickHotkey] = []string{
		"hints --action left_click --repeat --search",
	}

	elements := threeButtons(t)
	sim := newSimHarness(t, cfg, elements)
	sim.textInput.Started = true

	sim.pressHotkey(searchClickHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("search field opened", func() bool { return sim.textInput.StartCount() == 1 })

	callbacks := sim.textInput.Callbacks()
	callbacks.OnQueryChanged("cancel")
	sim.waitFor("hints narrowed to the query", func() bool {
		return len(sim.overlay.lastHintLabels()) == 1
	})

	confirmed := make(chan struct{})

	go func() {
		callbacks.OnConfirm()
		close(confirmed)
	}()

	select {
	case <-confirmed:
	case <-time.After(5 * time.Second):
		t.Fatal("confirming the search field did not return")
	}

	target := elements[1].Center()

	clicks := sim.ax.recordedClicks()
	if len(clicks) != 1 || clicks[0].point != target {
		t.Fatalf(
			"clicks after confirming = %+v, want one at the searched element %v",
			clicks,
			target,
		)
	}

	if got := sim.textInput.StartCount(); got != 2 {
		t.Errorf("search fields opened = %d, want 2, since the repeat reopens one", got)
	}
}

// TestSimulation_HintSearchNarrowsByTrailingLabel covers typing a label after
// the search text. "s" matches Save and Search, the space keeps both, and the
// label typed after it leaves one for Return to select. Each of the two labels
// lands on a different button.
func TestSimulation_HintSearchNarrowsByTrailingLabel(t *testing.T) {
	elements := threeButtons(t)
	saveCenter, searchCenter := elements[0].Center(), elements[2].Center()

	landed := map[image.Point]bool{}

	for pick := range 2 {
		sim := newSimHarness(t, simConfig(), elements)

		sim.pressHotkey(hintsHotkey)
		sim.waitMode(domain.ModeHints)
		sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

		sim.press("/")
		sim.waitFor("search input asked for", func() bool {
			return sim.overlay.searchInputAskCount() > 0
		})

		sim.press("s")
		sim.waitFor("hints narrowed to the text", func() bool {
			return len(sim.overlay.lastHintLabels()) == 2
		})

		sim.press(config.KeyNameSpace)

		labels := sim.overlay.lastHintLabels()
		if len(labels) != 2 {
			t.Fatalf("hints after the space = %v, want both text matches kept", labels)
		}

		sim.typeLabel(labels[pick])
		sim.waitFor("hints narrowed to the label", func() bool {
			return len(sim.overlay.lastHintLabels()) == 1
		})

		sim.press("Return")

		landed[sim.cursor.position()] = true
	}

	if len(landed) != 2 || !landed[saveCenter] || !landed[searchCenter] {
		t.Fatalf(
			"cursor landings = %v, want one on Save at %v and one on Search at %v",
			landed,
			saveCenter,
			searchCenter,
		)
	}
}

// TestSimulation_HintSearchConfirmClearsLabelHighlight covers Return when the
// first character of a label still leaves several matches. With --action, the
// search closes and the matches keep their labels for the user to type. The
// label input starts empty there, so the search's highlight comes off, and
// typing the whole label clicks.
func TestSimulation_HintSearchConfirmClearsLabelHighlight(t *testing.T) {
	const searchClickHotkey = "Primary+Shift+S"

	cfg := simConfig()
	cfg.Hotkeys.Bindings[searchClickHotkey] = []string{"hints --action left_click --search"}

	elements := manyButtons(t, 13)
	sim := newSimHarness(t, cfg, elements)

	sim.pressHotkey(searchClickHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("search input asked for", func() bool {
		return sim.overlay.searchInputAskCount() > 0
	})

	// Find a first character that more than one label starts with.
	starts := map[string][]string{}
	for _, label := range sim.overlay.lastHintLabels() {
		first := label[:1]
		starts[first] = append(starts[first], label)
	}

	var shared []string

	for _, labels := range starts {
		if len(labels) > 1 {
			shared = labels

			break
		}
	}

	if shared == nil {
		t.Fatalf("no two labels share a first character: %v", starts)
	}

	sim.press(config.KeyNameSpace)
	sim.typeLabel(shared[0][:1])
	sim.waitFor("hints narrowed to the label start", func() bool {
		return len(sim.overlay.lastHintLabels()) == len(shared)
	})

	sim.press("Return")

	for label, prefix := range sim.overlay.lastHintMatchedPrefixes() {
		if prefix != "" {
			t.Fatalf("label %q still highlights %q after the search closed", label, prefix)
		}
	}

	sim.typeLabel(shared[0])
	sim.waitFor("typed label clicked", func() bool { return len(sim.ax.recordedClicks()) == 1 })
}

// TestSimulation_HintSearchConfirmOnLabel covers --confirm-on-label. "cancel"
// leaves one match by text alone, which still waits for Return. "s" then a
// label leaves one match by label, which clicks without Return.
func TestSimulation_HintSearchConfirmOnLabel(t *testing.T) {
	const confirmHotkey = "Primary+Shift+L"

	cfg := simConfig()
	cfg.Hotkeys.Bindings[confirmHotkey] = []string{
		"hints --action left_click --search --confirm-on-label",
	}

	elements := threeButtons(t)
	sim := newSimHarness(t, cfg, elements)

	sim.pressHotkey(confirmHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("search input asked for", func() bool {
		return sim.overlay.searchInputAskCount() > 0
	})

	sim.typeLabel("cancel")
	sim.waitFor("hints narrowed to the text", func() bool {
		return len(sim.overlay.lastHintLabels()) == 1
	})

	if clicks := sim.ax.recordedClicks(); len(clicks) != 0 {
		t.Fatalf("clicks after a single text match = %+v, want none before Return", clicks)
	}

	for range "cancel" {
		sim.press(config.KeyNameDelete)
	}

	sim.press("s", config.KeyNameSpace)
	sim.waitFor("both s matches kept after the space", func() bool {
		return len(sim.overlay.lastHintLabels()) == 2
	})

	sim.typeLabel(sim.overlay.lastHintLabels()[0])

	clicks := sim.ax.recordedClicks()
	if len(clicks) != 1 {
		t.Fatalf("clicks after the label = %+v, want one without Return", clicks)
	}

	if point := clicks[0].point; point != elements[0].Center() && point != elements[2].Center() {
		t.Fatalf("click at %v, want Save or Search, the two s matches", point)
	}
}

// TestSimulation_SearchHintsActionConfirmOnLabel covers search_hints
// --confirm-on-label bound to "/" in hints mode. The mode was opened without
// the flag, so the click without Return comes from the binding.
func TestSimulation_SearchHintsActionConfirmOnLabel(t *testing.T) {
	const clickHotkey = "Primary+Shift+L"

	cfg := simConfig()
	cfg.Hotkeys.Bindings[clickHotkey] = []string{"hints --action left_click --repeat"}
	cfg.Hints.Hotkeys["/"] = config.StringOrStringArray{"action search_hints --confirm-on-label"}

	elements := threeButtons(t)
	sim := newSimHarness(t, cfg, elements)

	sim.pressHotkey(clickHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	sim.press("/")
	sim.waitFor("search input asked for", func() bool {
		return sim.overlay.searchInputAskCount() > 0
	})

	sim.press("s", config.KeyNameSpace)
	sim.waitFor("both s matches kept after the space", func() bool {
		return len(sim.overlay.lastHintLabels()) == 2
	})

	sim.typeLabel(sim.overlay.lastHintLabels()[0])

	sim.waitFor("label clicked without Return", func() bool {
		return len(sim.ax.recordedClicks()) == 1
	})
}

// TestSimulation_HintSearchFieldConfirmOnLabel covers --confirm-on-label in a
// platform search field, where the query arrives through its callback. With
// --repeat, confirming clicks and reopens hints with a new field from inside
// the old field's callback, which has to return.
func TestSimulation_HintSearchFieldConfirmOnLabel(t *testing.T) {
	const confirmHotkey = "Primary+Shift+L"

	cfg := simConfig()
	cfg.Hotkeys.Bindings[confirmHotkey] = []string{
		"hints --action left_click --repeat --search --confirm-on-label",
	}

	elements := threeButtons(t)
	sim := newSimHarness(t, cfg, elements)
	sim.textInput.Started = true

	sim.pressHotkey(confirmHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("search field opened", func() bool { return sim.textInput.StartCount() == 1 })

	callbacks := sim.textInput.Callbacks()
	callbacks.OnQueryChanged("s ")
	sim.waitFor("both s matches kept after the space", func() bool {
		return len(sim.overlay.lastHintLabels()) == 2
	})

	query := "s " + strings.ToLower(sim.overlay.lastHintLabels()[0])
	changed := make(chan struct{})

	go func() {
		callbacks.OnQueryChanged(query)
		close(changed)
	}()

	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("typing the label into the search field did not return")
	}

	if clicks := sim.ax.recordedClicks(); len(clicks) != 1 {
		t.Fatalf("clicks after the label = %+v, want one without confirming", clicks)
	}

	if got := sim.textInput.StartCount(); got != 2 {
		t.Errorf("search fields opened = %d, want 2, since the repeat reopens one", got)
	}
}

// TestSimulation_HintsTwoCharLabels covers label generation past the
// single-character alphabet, since 12 elements with 9 hint characters force
// two-character labels. A first keystroke narrows the drawn set, and the
// full label still lands the cursor on an element.
func TestSimulation_HintsTwoCharLabels(t *testing.T) {
	elements := manyButtons(t, 12)
	sim := newSimHarness(t, simConfig(), elements)

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	labels := sim.overlay.lastHintLabels()
	if len(labels) != len(elements) {
		t.Fatalf("expected %d labels, got %d", len(elements), len(labels))
	}

	// With 9 hint characters and 12 elements the generator must overflow into
	// multi-character labels for the tail of the set.
	label := ""

	for _, candidate := range labels {
		if len(candidate) >= 2 {
			label = strings.ToLower(candidate)

			break
		}
	}

	if label == "" {
		t.Fatalf(
			"expected at least one multi-character label for %d elements, got %v",
			len(elements),
			labels,
		)
	}

	// First character narrows the visible hint set.
	sim.press(string(label[0]))
	sim.waitFor("hints narrowed by prefix", func() bool {
		remaining := sim.overlay.lastHintLabels()

		return len(remaining) > 0 && len(remaining) < len(elements)
	})

	// Backspace restores the full set.
	sim.press("Backspace")
	sim.waitFor("hints restored after backspace", func() bool {
		return len(sim.overlay.lastHintLabels()) == len(elements)
	})

	// The full label selects an element.
	movesBefore := sim.cursor.moveCount()
	sim.typeLabel(label)
	sim.waitFor("cursor moved to the selected element", func() bool {
		return sim.cursor.moveCount() > movesBefore
	})

	target := sim.cursor.position()
	for _, elem := range elements {
		if target == elem.Center() {
			return
		}
	}

	t.Fatalf("cursor landed at %v, not on any element center", target)
}

// TestSimulation_HintsInModeClick covers the two-step click a user performs
// with the default binding: select a hint (cursor moves, hints re-arm), then
// Shift+L clicks at the cursor.
func TestSimulation_HintsInModeClick(t *testing.T) {
	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, simConfig(), []*element.Element{save})

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	sim.typeLabel(sim.overlay.lastHintLabels()[0])
	sim.waitFor("cursor on element", func() bool {
		return sim.cursor.position() == save.Center()
	})

	sim.press("Shift+L")

	sim.waitFor("click recorded", func() bool { return len(sim.ax.recordedClicks()) > 0 })

	click := sim.ax.recordedClicks()[0]
	if click.action != action.TypeLeftClick {
		t.Fatalf("expected left click, got action type %v", click.action)
	}

	if click.point != save.Center() {
		t.Fatalf("click at %v, expected %v", click.point, save.Center())
	}
}

// TestSimulation_HintsRightClickAction covers a non-default pending action.
// "hints right_click" must produce a right click at the element center.
func TestSimulation_HintsRightClickAction(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{"hints right_click"}

	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, cfg, []*element.Element{save})

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	sim.typeLabel(sim.overlay.lastHintLabels()[0])

	sim.waitFor("click recorded", func() bool { return len(sim.ax.recordedClicks()) > 0 })

	click := sim.ax.recordedClicks()[0]
	if click.action != action.TypeRightClick {
		t.Fatalf("expected right click, got action type %v", click.action)
	}

	if click.point != save.Center() {
		t.Fatalf("click at %v, expected %v", click.point, save.Center())
	}
}

// TestSimulation_HintsRepeatJourney covers --repeat. After each click the
// mode re-arms with the action preserved, so consecutive selections click
// consecutively without re-pressing the hotkey.
func TestSimulation_HintsRepeatJourney(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{stepHintsRepeatClicks}

	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, cfg, []*element.Element{save})

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	drawsBefore := sim.overlay.hintDrawCount()
	sim.typeLabel(sim.overlay.lastHintLabels()[0])

	sim.waitFor("first click recorded", func() bool { return len(sim.ax.recordedClicks()) == 1 })
	sim.waitFor("hints re-armed after repeat click", func() bool {
		return sim.app.CurrentMode() == domain.ModeHints &&
			sim.overlay.hintDrawCount() > drawsBefore
	})

	sim.typeLabel(sim.overlay.lastHintLabels()[0])
	sim.waitFor("second click recorded", func() bool { return len(sim.ax.recordedClicks()) == 2 })

	for _, click := range sim.ax.recordedClicks() {
		if click.point != save.Center() {
			t.Fatalf("repeat click at %v, expected %v", click.point, save.Center())
		}
	}

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_HintsRepeatExitOnUnmatchedJourney runs hints with --action left_click
// --repeat --exit-on-unmatched. Clicking a hint runs the action and re-activates hints
// mode, and a later unmatched key returns to idle mode.
func TestSimulation_HintsRepeatExitOnUnmatchedJourney(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{
		"hints --action left_click --repeat --exit-on-unmatched",
	}

	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, cfg, []*element.Element{save})

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	drawsBefore := sim.overlay.hintDrawCount()
	sim.typeLabel(sim.overlay.lastHintLabels()[0])

	sim.waitFor("first click recorded", func() bool { return len(sim.ax.recordedClicks()) == 1 })
	sim.waitFor("hints re-armed after repeat click", func() bool {
		return sim.app.CurrentMode() == domain.ModeHints &&
			sim.overlay.hintDrawCount() > drawsBefore
	})

	if clicks := sim.ax.recordedClicks(); len(clicks) != 1 || clicks[0].point != save.Center() {
		t.Fatalf("click recorded at %v, expected %v", clicks, save.Center())
	}

	sim.press("z")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_HintsFlaggedBindingJourney covers a binding that carries a
// whole activation rather than a bare action: what to do, what to hold while
// doing it, and what to run once it is done.
//
// Each flag is asserted where a user would notice it, which is which button
// was clicked, what was held, and what ran afterwards. A flag that parses and then
// goes missing between the binding and the mode is invisible to the grammar's
// own tests, because the command still reads correctly and the mode still
// activates. This is the seam where it shows up.
func TestSimulation_HintsFlaggedBindingJourney(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{
		"hints --action=right_click --modifier=shift --on-exit='action left_click'",
	}

	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, cfg, []*element.Element{save})

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	sim.typeLabel(sim.overlay.lastHintLabels()[0])

	// The selection performs the action, and the exit step then runs where the
	// selection left the cursor.
	sim.waitFor("the action and its exit step both ran", func() bool {
		return len(sim.ax.recordedClicks()) == 2
	})

	clicks := sim.ax.recordedClicks()

	selection, onExit := clicks[0], clicks[1]

	if selection.action != action.TypeRightClick {
		t.Errorf("--action=right_click produced %v", selection.action)
	}

	if selection.modifiers != action.ModShift {
		t.Errorf("--modifier=shift produced modifiers %v", selection.modifiers)
	}

	if selection.point != save.Center() {
		t.Errorf("click landed at %v, expected element center %v",
			selection.point, save.Center())
	}

	if onExit.action != action.TypeLeftClick {
		t.Errorf("--on-exit='action left_click' produced %v", onExit.action)
	}

	if onExit.point != save.Center() {
		t.Errorf("the exit step clicked at %v, expected the selection %v",
			onExit.point, save.Center())
	}

	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_HintsRunAgainKeepsTheSessionsFilters covers `neru hints`
// run with no flags while a session started with --role and --text is open. A
// flag left off a re-activation keeps the session's value, so the rescan gets
// the same role and text filter as the scan that opened the mode.
func TestSimulation_HintsRunAgainKeepsTheSessionsFilters(t *testing.T) {
	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, simConfig(), []*element.Element{save})

	for _, args := range [][]string{{"--role=button", "--text=Save"}, nil} {
		resp := sim.app.HandleCommand(context.Background(), ipc.Command{
			Action: domain.ModeNameHints,
			Args:   args,
		})
		if !resp.Success {
			t.Fatalf("hints %v failed: %s (%s)", args, resp.Message, resp.Code)
		}

		sim.waitMode(domain.ModeHints)
	}

	filters := sim.ax.scanFilters()
	if len(filters) != 2 {
		t.Fatalf("got %d scans, want the activation and the re-activation", len(filters))
	}

	opened, rerun := filters[0], filters[1]

	if !slices.Equal(rerun.Roles, opened.Roles) {
		t.Errorf("re-activation scanned roles %v, want the session's %v", rerun.Roles, opened.Roles)
	}

	if rerun.TitleContains != opened.TitleContains {
		t.Errorf("re-activation filtered text %q, want the session's %q",
			rerun.TitleContains, opened.TitleContains)
	}
}

// TestSimulation_HintsMoveMouseThenScroll covers picking a place without
// pressing anything there. Selecting the hint leaves the cursor on the
// element, and the exit step opens scroll mode at that point.
func TestSimulation_HintsMoveMouseThenScroll(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{"hints --action=move_mouse --on-exit=scroll"}

	pane := simElement(t, "pane", image.Rect(100, 100, 220, 140), "Pane")
	sim := newSimHarness(t, cfg, []*element.Element{pane})

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	sim.typeLabel(sim.overlay.lastHintLabels()[0])

	sim.waitMode(domain.ModeScroll)

	if got := sim.cursor.position(); got != pane.Center() {
		t.Errorf("cursor at %v in scroll mode, expected element center %v", got, pane.Center())
	}

	for _, performed := range sim.ax.recordedClicks() {
		if performed.action.IsMouseButton() {
			t.Errorf("move_mouse pressed %v at %v", performed.action, performed.point)
		}
	}
}

// TestSimulation_RefusedBindingActivatesNothing covers the other half. A
// binding the grammar refuses enters no mode and draws nothing.
//
// Both refusals are ones a user used to experience as Neru being unreliable
// rather than as a mistake they had made. A typo activated the mode and did
// nothing else. "grid --search" activated grid and dropped the flag in silence,
// so the search input a user was waiting for never appeared. Refusing the whole
// command is what makes that a mistake somebody can find.
func TestSimulation_RefusedBindingActivatesNothing(t *testing.T) {
	tests := map[string]struct {
		hotkey  string
		binding string
		mode    domain.Mode
	}{
		"a mistyped flag":                 {hintsHotkey, "hints --sarch", domain.ModeHints},
		"a flag the mode does not accept": {gridHotkey, "grid --search", domain.ModeGrid},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := simConfig()
			cfg.Hotkeys.Bindings[testCase.hotkey] = []string{testCase.binding}

			sim := newSimHarness(t, cfg, threeButtons(t))

			sim.pressHotkey(testCase.hotkey)
			sim.neverMode(testCase.mode, 250*time.Millisecond)

			if sim.overlay.isVisible() {
				t.Errorf("%q showed the overlay; a refused command activates nothing",
					testCase.binding)
			}

			if sim.overlay.hintDrawCount() != 0 || sim.overlay.lastGrid() != nil {
				t.Errorf("%q drew a mode; a refused command activates nothing",
					testCase.binding)
			}
		})
	}
}

// TestSimulation_RecursiveGridJourney covers the recursive grid. Activation
// draws the full-screen grid, and choosing a cell redraws zoomed into that
// cell's bounds.
func TestSimulation_RecursiveGridJourney(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(recursiveGridHotkey)
	sim.waitMode(domain.ModeRecursiveGrid)

	sim.waitFor("recursive grid drawn", func() bool {
		_, ok := sim.overlay.lastRecursiveGridBounds()

		return ok
	})

	// "r" is the top-left cell of the default 3x3 key layout.
	sim.press("r")

	topLeftThird := image.Rect(0, 0, simScreen.Dx()/3+1, simScreen.Dy()/3+1)
	sim.waitFor("grid zoomed into the top-left cell", func() bool {
		bounds, ok := sim.overlay.lastRecursiveGridBounds()

		return ok && bounds.In(topLeftThird)
	})

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_BisectJourney covers bisect mode end to end. Activation
// draws the whole screen divided in four with the quadrant keys, each cut
// redraws the half or quadrant kept and moves the cursor to its center,
// Backspace takes a cut back, and Escape lands in idle with the cursor where
// the last cut put it.
func TestSimulation_BisectJourney(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(bisectHotkey)
	sim.waitMode(domain.ModeBisect)

	sim.waitFor("bisect drawn over the whole screen", func() bool {
		frame, ok := sim.overlay.lastBisectFrame()

		return ok && frame.Bounds.Eq(simScreen) && frame.Keys == "yubn"
	})

	sim.waitFor("cursor at the screen center", func() bool {
		return sim.cursor.position() == image.Pt(960, 540)
	})

	// Activation is one draw: the transition, not a redraw followed by it.
	if got := sim.overlay.bisectDrawCount(); got != 1 {
		t.Fatalf("activation drew the region %d times, want 1", got)
	}

	sim.press("l")
	sim.waitFor("cut right kept the right half", func() bool {
		frame, ok := sim.overlay.lastBisectFrame()

		return ok && frame.Bounds.Eq(image.Rect(960, 0, 1920, 1080)) &&
			sim.cursor.position() == image.Pt(1440, 540)
	})

	sim.press("y")
	sim.waitFor("cut up-left kept that quadrant of the half", func() bool {
		frame, ok := sim.overlay.lastBisectFrame()

		return ok && frame.Bounds.Eq(image.Rect(960, 0, 1440, 540))
	})

	sim.press("Backspace")
	sim.waitFor("backspace restored the right half", func() bool {
		frame, ok := sim.overlay.lastBisectFrame()

		return ok && frame.Bounds.Eq(image.Rect(960, 0, 1920, 1080)) &&
			sim.cursor.position() == image.Pt(1440, 540)
	})

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	if got := sim.cursor.position(); got != image.Pt(1440, 540) {
		t.Fatalf("leaving bisect moved the cursor to %v", got)
	}
}

// TestSimulation_BisectStartsFromTheFocusedWindow covers the window scope.
// With --capture-scope window the region starts as the focused window's
// bounds, placed where the window is rather than in the screen's corner, and
// the cursor goes to that window's center. The default scope, screen, ignores
// the window.
func TestSimulation_BisectStartsFromTheFocusedWindow(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[bisectWindowHotkey] = []string{"bisect --capture-scope window"}

	sim := newSimHarness(t, cfg, nil)

	window := image.Rect(400, 200, 1200, 900)
	sim.desktop.focusWindow(window)

	sim.pressHotkey(bisectWindowHotkey)
	sim.waitMode(domain.ModeBisect)

	sim.waitFor("bisect drawn over the focused window where it is", func() bool {
		frame, ok := sim.overlay.lastBisectFrame()

		return ok && frame.Bounds.Eq(window) && sim.cursor.position() == image.Pt(800, 550)
	})

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	sim.pressHotkey(bisectHotkey)
	sim.waitMode(domain.ModeBisect)

	sim.waitFor("the default scope starts from the screen", func() bool {
		frame, ok := sim.overlay.lastBisectFrame()

		return ok && frame.Bounds.Eq(simScreen)
	})

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_CaptureScopeCycleStepsWhileTheModeIsOpen covers a cycle list.
// Entering bisect with --capture-scope window,screen takes the first entry,
// the same command while bisect is open takes the one after the scope in
// use, and the list wraps.
func TestSimulation_CaptureScopeCycleStepsWhileTheModeIsOpen(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[globalToggleChord] = []string{"bisect --capture-scope window,screen"}

	sim := newSimHarness(t, cfg, nil)

	window := image.Rect(400, 200, 1200, 900)
	sim.desktop.focusWindow(window)

	sim.pressHotkey(globalToggleChord)
	sim.waitMode(domain.ModeBisect)
	sim.waitFor("the mode taking keystrokes", sim.tap.IsEnabled)

	for _, step := range []struct {
		what string
		want image.Rectangle
	}{
		{"entering takes the first entry, the window", window},
		{"the command again steps to the screen", simScreen},
		{"and once more wraps back to the window", window},
	} {
		sim.waitFor(step.what, func() bool {
			frame, ok := sim.overlay.lastBisectFrame()

			return ok && frame.Bounds.Eq(step.want)
		})

		sim.press(globalToggleChord)
	}

	if mode := sim.app.CurrentMode(); mode != domain.ModeBisect {
		t.Fatalf("cycling the scope left bisect for %v", mode)
	}
}

// TestSimulation_CaptureScopeCycleWithToggleExitsAfterTheLastEntry covers a
// cycle list with --toggle. The list steps as it does without the flag until
// the last entry. The press there exits instead of wrapping, and the next
// press starts again from the first entry.
func TestSimulation_CaptureScopeCycleWithToggleExitsAfterTheLastEntry(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[globalToggleChord] = []string{
		"bisect --capture-scope window,screen --toggle",
	}

	sim := newSimHarness(t, cfg, nil)

	window := image.Rect(400, 200, 1200, 900)
	sim.desktop.focusWindow(window)

	for range 2 {
		sim.pressHotkey(globalToggleChord)
		sim.waitMode(domain.ModeBisect)
		sim.waitFor("the mode taking keystrokes", sim.tap.IsEnabled)

		for _, step := range []struct {
			what string
			want image.Rectangle
		}{
			{"entering takes the first entry, the window", window},
			{"the command again steps to the screen", simScreen},
		} {
			sim.waitFor(step.what, func() bool {
				frame, ok := sim.overlay.lastBisectFrame()

				return ok && frame.Bounds.Eq(step.want)
			})

			sim.press(globalToggleChord)
		}

		sim.waitMode(domain.ModeIdle)
	}
}

// TestSimulation_GridCoversTheFocusedWindow covers the window scope for grid.
// With --capture-scope window the grid covers the focused window's bounds,
// placed where the window is, and a selected cell lands inside it.
// The default scope, screen, ignores the window.
func TestSimulation_GridCoversTheFocusedWindow(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[windowScopeHotkey] = []string{"grid --capture-scope window"}

	sim := newSimHarness(t, cfg, nil)

	window := image.Rect(400, 200, 1200, 900)
	sim.desktop.focusWindow(window)

	sim.pressHotkey(windowScopeHotkey)
	sim.waitMode(domain.ModeGrid)

	sim.waitFor("grid drawn over the focused window where it is", func() bool {
		grid := sim.overlay.lastGrid()

		return grid != nil && grid.Bounds().Eq(window)
	})

	cells := sim.overlay.lastGrid().Cells()
	cell := cells[len(cells)/2]

	movesBefore := sim.cursor.moveCount()
	sim.typeLabel(cell.Coordinate())

	sim.waitFor("cursor moved into the selected cell", func() bool {
		return sim.cursor.moveCount() > movesBefore
	})

	if pos := sim.cursor.position(); !pos.In(window) {
		t.Fatalf("cursor at %v landed outside the focused window %v", pos, window)
	}

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)

	sim.waitFor("the default scope covers the screen", func() bool {
		grid := sim.overlay.lastGrid()

		return grid != nil && grid.Bounds().Eq(simScreen)
	})

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_RecursiveGridStartsFromTheFocusedWindow covers the window
// scope for recursive grid. With --capture-scope window the first level is
// the focused window's bounds, placed where the window is, and a cell chosen
// at that level is a third of the window rather than of the screen.
func TestSimulation_RecursiveGridStartsFromTheFocusedWindow(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[windowScopeHotkey] = []string{"recursive_grid --capture-scope window"}

	sim := newSimHarness(t, cfg, nil)

	window := image.Rect(300, 300, 1200, 900)
	sim.desktop.focusWindow(window)

	sim.pressHotkey(windowScopeHotkey)
	sim.waitMode(domain.ModeRecursiveGrid)

	sim.waitFor("recursive grid drawn over the focused window where it is", func() bool {
		bounds, ok := sim.overlay.lastRecursiveGridBounds()

		return ok && bounds.Eq(window)
	})

	// "r" is the top-left cell of the default 3x3 key layout.
	sim.press("r")

	topLeftThird := image.Rect(
		window.Min.X, window.Min.Y,
		window.Min.X+window.Dx()/3+1, window.Min.Y+window.Dy()/3+1,
	)
	sim.waitFor("grid zoomed into the window's top-left cell", func() bool {
		bounds, ok := sim.overlay.lastRecursiveGridBounds()

		return ok && bounds.In(topLeftThird) && bounds.Min.Eq(window.Min)
	})

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	sim.pressHotkey(recursiveGridHotkey)
	sim.waitMode(domain.ModeRecursiveGrid)

	sim.waitFor("the default scope starts from the screen", func() bool {
		bounds, ok := sim.overlay.lastRecursiveGridBounds()

		return ok && bounds.Eq(simScreen)
	})

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_BisectHoldMode covers --cursor-selection-mode hold. Cuts
// narrow the region and draw the pointer stand-in at its center while the
// real cursor stays where it was, and the backtick toggle brings the cursor
// onto the selection.
func TestSimulation_BisectHoldMode(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[bisectWindowHotkey] = []string{"bisect --cursor-selection-mode hold"}

	sim := newSimHarness(t, cfg, nil)
	start := sim.cursor.position()

	sim.pressHotkey(bisectWindowHotkey)
	sim.waitMode(domain.ModeBisect)

	sim.press("l")
	sim.waitFor("cut drawn with the pointer stand-in, cursor held", func() bool {
		frame, ok := sim.overlay.lastBisectFrame()

		return ok && frame.Bounds.Eq(image.Rect(960, 0, 1920, 1080)) &&
			frame.Pointer.Visible && frame.Pointer.Position == image.Pt(1440, 540) &&
			sim.cursor.position() == start
	})

	sim.press("`")
	sim.waitFor("toggle brought the cursor onto the selection", func() bool {
		frame, ok := sim.overlay.lastBisectFrame()

		return ok && !frame.Pointer.Visible && sim.cursor.position() == image.Pt(1440, 540)
	})

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_BisectWindowScopeOnASecondDisplay covers the placement on a
// display that does not start at the origin. The window's region lands where
// the window is on that display, not offset by the display's position.
func TestSimulation_BisectWindowScopeOnASecondDisplay(t *testing.T) {
	cfg := simConfig()
	cfg.Bisect.CaptureScope = domain.CaptureScopeWindow

	sim := newSimHarness(t, cfg, nil)

	second := image.Rect(1920, 0, 3840, 1080)
	sim.desktop.set([]simDisplay{
		{name: "main", bounds: simScreen},
		{name: "second", bounds: second},
	})
	sim.cursor.moveTo(image.Pt(2500, 500))

	window := image.Rect(2200, 100, 3000, 700)
	sim.desktop.focusWindow(window)

	sim.pressHotkey(bisectHotkey)
	sim.waitMode(domain.ModeBisect)

	// The frame is drawn in that display's own space, and the cursor lands
	// at the window's center in global space.
	sim.waitFor("region placed relative to the second display", func() bool {
		frame, ok := sim.overlay.lastBisectFrame()

		return ok && frame.Bounds.Eq(window.Sub(second.Min)) &&
			sim.cursor.position() == image.Pt(2600, 400)
	})

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_ScrollDirections covers all four scroll directions with
// their default keys. Each keypress must scroll on the right axis in the
// right direction.
func TestSimulation_ScrollDirections(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	keys := []string{"j", "k", "h", "l"}
	for i, key := range keys {
		sim.press(key)

		expected := i + 1
		sim.waitFor("scroll for "+key, func() bool {
			return len(sim.ax.recordedScrolls()) >= expected
		})
	}

	scrolls := sim.ax.recordedScrolls()
	if len(scrolls) < 4 {
		t.Fatalf("expected 4 scrolls, got %d", len(scrolls))
	}

	down, up, left, right := scrolls[0], scrolls[1], scrolls[2], scrolls[3]

	if down.Y == 0 || up.Y == 0 || (down.Y > 0) == (up.Y > 0) {
		t.Fatalf("j and k must scroll vertically in opposite directions, got %v and %v", down, up)
	}

	if left.X == 0 || right.X == 0 || (left.X > 0) == (right.X > 0) {
		t.Fatalf(
			"h and l must scroll horizontally in opposite directions, got %v and %v",
			left,
			right,
		)
	}

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_ScrollTopSequence covers the two-key "gg" sequence binding.
func TestSimulation_ScrollTopSequence(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	sim.press("g", "g")

	sim.waitFor("go-top scroll recorded", func() bool {
		return len(sim.ax.recordedScrolls()) > 0
	})
}

// TestSimulation_HotkeyRegistration covers startup registration. Every
// binding in the config must be registered with the hotkey backend,
// canonicalized for the platform.
func TestSimulation_HotkeyRegistration(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	bindings := []string{hintsHotkey, gridHotkey, recursiveGridHotkey, scrollHotkey}
	for _, binding := range bindings {
		canonical := config.CanonicalHotkeyForPlatform(binding)
		sim.waitFor("registration of "+canonical, func() bool {
			return sim.hotkeys.callbackFor(canonical) != nil
		})
	}
}

// TestSimulation_DisabledModeBindingSkipped covers config gating. A binding
// whose mode is disabled must never be registered, while others still are.
func TestSimulation_DisabledModeBindingSkipped(t *testing.T) {
	cfg := simConfig()
	cfg.Grid.Enabled = false

	sim := newSimHarness(t, cfg, nil)

	sim.waitFor("hints chord registered", func() bool {
		return sim.hotkeys.callbackFor(config.CanonicalHotkeyForPlatform(hintsHotkey)) != nil
	})

	if sim.app.GridEnabled() {
		t.Fatal("GridEnabled() must report false when disabled in config")
	}

	if !sim.app.HintsEnabled() {
		t.Fatal("HintsEnabled() must report true by default")
	}

	gridChord := config.CanonicalHotkeyForPlatform(gridHotkey)
	if sim.hotkeys.callbackFor(gridChord) != nil {
		t.Fatalf("chord %s must not be registered while grid mode is disabled", gridChord)
	}
}

// TestSimulation_PassthroughExitsMode covers exit_after_passthrough. When a
// modifier shortcut passes through to the OS, the active mode exits.
func TestSimulation_PassthroughExitsMode(t *testing.T) {
	cfg := simConfig()
	cfg.General.PassthroughUnboundedKeys = true
	cfg.General.ShouldExitAfterPassthrough = true

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	sim.waitFor("passthrough callback registered", func() bool {
		return sim.tap.PassthroughCallback() != nil
	})

	sim.tap.TriggerPassthrough()
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_StalePassthroughCallback pins the regression where a
// passthrough callback captured by an old mode fired after a mode switch and
// wrongly exited the new mode.
func TestSimulation_StalePassthroughCallback(t *testing.T) {
	cfg := simConfig()
	cfg.General.PassthroughUnboundedKeys = true
	cfg.General.ShouldExitAfterPassthrough = true

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	sim.waitFor("scroll passthrough callback registered", func() bool {
		return sim.tap.PassthroughCallback() != nil
	})

	staleCallback := sim.tap.PassthroughCallback()

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)

	staleCallback()
	sim.neverMode(domain.ModeIdle, 150*time.Millisecond)

	if sim.app.CurrentMode() != domain.ModeGrid {
		t.Fatalf("stale passthrough callback changed mode to %v, want grid", sim.app.CurrentMode())
	}

	sim.tap.TriggerPassthrough()
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_SystrayComponent covers systray wiring: present by default,
// absent when disabled.
func TestSimulation_SystrayComponent(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)
	if sim.app.GetSystrayComponent() == nil {
		t.Error("expected systray component when enabled (default)")
	}

	cfg := simConfig()
	cfg.Systray.Enabled = false

	simDisabled := newSimHarness(t, cfg, nil)
	if simDisabled.app.GetSystrayComponent() != nil {
		t.Error("expected no systray component when disabled")
	}
}

const monitorSelectHotkey = "Primary+Shift+M"

// The two fixture displays a monitor_select journey picks between.
const (
	mainDisplayName   = "MainDisplay"
	secondDisplayName = "SecondDisplay"
)

// monitorSelectConfig returns a config with monitor_select enabled and bound.
func monitorSelectConfig() *config.Config {
	cfg := simConfig()
	cfg.MonitorSelect.Enabled = true
	cfg.Hotkeys.Bindings[monitorSelectHotkey] = []string{"monitor_select"}

	return cfg
}

// TestSimulation_MonitorSelectJourney covers picking a monitor on a
// two-display desktop. Activation draws one labeled panel per monitor, and
// typing a label moves the cursor to that monitor's center and exits.
func TestSimulation_MonitorSelectJourney(t *testing.T) {
	cfg := monitorSelectConfig()

	second := image.Rect(1920, 0, 3840, 1080)
	sim := newSimHarnessWithDisplays(t, cfg, nil, []simDisplay{
		{name: mainDisplayName, bounds: simScreen},
		{name: secondDisplayName, bounds: second},
	})

	sim.pressHotkey(monitorSelectHotkey)
	sim.waitMode(domain.ModeMonitorSelect)

	sim.waitFor("monitor panels drawn", func() bool {
		return len(sim.overlay.lastMonitorTargets()) == 2
	})

	secondLabel := ""

	for _, target := range sim.overlay.lastMonitorTargets() {
		if target.Bounds == second {
			secondLabel = target.Label
		}
	}

	if secondLabel == "" {
		t.Fatalf(
			"no monitor target with bounds %v in %v",
			second, sim.overlay.lastMonitorTargets(),
		)
	}

	sim.typeLabel(secondLabel)

	sim.waitMode(domain.ModeIdle)

	secondCenter := image.Point{X: 2880, Y: 540}
	sim.waitFor("cursor on the second monitor", func() bool {
		return sim.cursor.position() == secondCenter
	})

	// Exit must tear the panels down, or they would linger on screen.
	sim.waitFor("monitor panels hidden after exit", func() bool {
		return len(sim.overlay.drawnModeNames()) == 0
	})
}

// TestSimulation_MonitorSelectJourney_DisplaysSharingAName uses the layout
// that lost a monitor. The platform reports two displays under one name, as
// Windows does with "Generic PnP Monitor", and the left one sits at a
// negative X. Every display must get its own panel, and picking the left one
// must move the cursor there. When targets were resolved by name, both took
// the bounds of the first display, so only its label was drawn and the other
// monitor could not be reached.
func TestSimulation_MonitorSelectJourney_DisplaysSharingAName(t *testing.T) {
	cfg := monitorSelectConfig()

	const sharedName = "Generic PnP Monitor"

	left := image.Rect(-1600, 0, 0, 2560)
	sim := newSimHarnessWithDisplays(t, cfg, nil, []simDisplay{
		{name: sharedName, bounds: simScreen},
		{name: sharedName, bounds: left},
	})

	sim.pressHotkey(monitorSelectHotkey)
	sim.waitMode(domain.ModeMonitorSelect)

	sim.waitFor("one panel per display", func() bool {
		return len(sim.overlay.lastMonitorTargets()) == 2
	})

	leftLabel := ""

	for _, target := range sim.overlay.lastMonitorTargets() {
		if target.Bounds == left {
			leftLabel = target.Label
		}
	}

	if leftLabel == "" {
		t.Fatalf(
			"no panel on the left display %v in %v",
			left, sim.overlay.lastMonitorTargets(),
		)
	}

	sim.typeLabel(leftLabel)
	sim.waitMode(domain.ModeIdle)

	leftCenter := image.Point{X: -800, Y: 1280}
	sim.waitFor("cursor on the left display", func() bool {
		return sim.cursor.position() == leftCenter
	})
}

// TestSimulation_MonitorMoveNext_StepsThroughDisplaysSharingAName covers the
// relative move on the same layout. A step from the first display must reach
// the second even though both have one name, and a second step must return.
func TestSimulation_MonitorMoveNext_StepsThroughDisplaysSharingAName(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[moveMonitorHotkey] = []string{moveMonitorBinding}

	const sharedName = "Generic PnP Monitor"

	left := image.Rect(-1600, 0, 0, 2560)
	sim := newSimHarnessWithDisplays(t, cfg, nil, []simDisplay{
		{name: sharedName, bounds: simScreen},
		{name: sharedName, bounds: left},
	})

	sim.pressHotkey(moveMonitorHotkey)

	leftCenter := image.Point{X: -800, Y: 1280}
	sim.waitFor("cursor on the left display", func() bool {
		return sim.cursor.position() == leftCenter
	})

	sim.pressHotkey(moveMonitorHotkey)

	mainCenter := image.Point{
		X: simScreen.Min.X + simScreen.Dx()/2,
		Y: simScreen.Min.Y + simScreen.Dy()/2,
	}
	sim.waitFor("cursor back on the first display", func() bool {
		return sim.cursor.position() == mainCenter
	})
}

// TestSimulation_MonitorSelectNotSupported pins the platform-stub contract.
// On a backend without the MonitorSelector extension, activation reports
// CodeNotSupported, the mode never engages, and the app keeps working.
func TestSimulation_MonitorSelectNotSupported(t *testing.T) {
	sim := newSimHarnessHeadlessOverlay(t, monitorSelectConfig(), []simDisplay{
		{name: mainDisplayName, bounds: simScreen},
		{name: secondDisplayName, bounds: image.Rect(1920, 0, 3840, 1080)},
	})

	sim.pressHotkey(monitorSelectHotkey)
	sim.neverMode(domain.ModeMonitorSelect, 250*time.Millisecond)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
}

// TestSimulation_StickyModifierClick covers sticky modifiers. Tapping Shift
// arms it (indicator drawn), and the next hint-selection click carries the
// Shift modifier.
func TestSimulation_StickyModifierClick(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{"hints left_click"}

	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, cfg, []*element.Element{save})

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	// A quick modifier tap (down then up, no key in between) arms sticky Shift.
	sim.press("__modifier_shift_down", "__modifier_shift_up")

	sim.waitFor("sticky indicator drawn", func() bool {
		return sim.overlay.stickyIndicatorDrawCount() > 0
	})

	// With Shift held (sticky posts it physically), the event tap reports the
	// label keystroke as a Shift chord. Neru strips it for label matching.
	sim.press("Shift+" + strings.ToUpper(sim.overlay.lastHintLabels()[0]))

	sim.waitFor("click recorded", func() bool { return len(sim.ax.recordedClicks()) > 0 })

	click := sim.ax.recordedClicks()[0]
	if !click.modifiers.Has(action.ModShift) {
		t.Fatalf("expected the click to carry the sticky Shift modifier, got %v", click.modifiers)
	}

	if click.point != save.Center() {
		t.Fatalf("click at %v, expected %v", click.point, save.Center())
	}
}

// TestSimulation_StickyModifierScroll pins that a sticky modifier reaches a
// scroll the way it reaches a click. Tapping ctrl in scroll mode and then
// scrolling is the user asking for a ctrl+scroll, which most applications read
// as zoom, and the plain scroll that used to go out instead read as a broken
// binding.
func TestSimulation_StickyModifierScroll(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	sim.press("__modifier_ctrl_down", "__modifier_ctrl_up")

	sim.waitFor("sticky indicator drawn", func() bool {
		return sim.overlay.stickyIndicatorDrawCount() > 0
	})

	// Sticky posts ctrl physically, so the scroll key arrives as a ctrl chord
	// that Neru strips for binding resolution.
	sim.press("Ctrl+j")

	sim.waitFor("scroll recorded", func() bool { return len(sim.ax.recordedScrolls()) > 0 })

	if mods := sim.ax.recordedScrollModifiers()[0]; !mods.Has(action.ModCtrl) {
		t.Fatalf("expected the scroll to carry the sticky Ctrl modifier, got %v", mods)
	}
}

// TestSimulation_HeldRepeatScroll covers held-key repeat. Holding j keeps
// scrolling on the configured interval, and the key release stops it.
func TestSimulation_HeldRepeatScroll(t *testing.T) {
	const (
		repeatDelayMS    = 5
		repeatIntervalMS = 5
		// scrollsAwaited is enough repeats to say the engine is running rather
		// than that the key was handled once.
		scrollsAwaited = 4
		// stopSampleWindow is how long the stop check lets the engine run
		// between its two samples.
		stopSampleWindow = 100 * time.Millisecond
	)

	cfg := simConfig()
	cfg.HeldRepeat.Enabled = true
	cfg.HeldRepeat.InitialDelay = repeatDelayMS
	cfg.HeldRepeat.Interval = repeatIntervalMS

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	// Key down without a release: the repeat engine takes over.
	sim.press("j")

	// Unlike every other wait in these journeys, this one is waiting on a
	// timer. The first scroll cannot arrive before the initial delay and each
	// one after it costs an interval, however fast the machine is. That
	// duration is stated on top of the headroom rather than absorbed into it,
	// so a journey that turns the interval up does not silently eat the slack
	// every other wait is relying on.
	repeatRunUp := repeatDelayMS*time.Millisecond +
		(scrollsAwaited-1)*repeatIntervalMS*time.Millisecond

	sim.waitForWithin(
		simWaitHeadroom+repeatRunUp,
		fmt.Sprintf("held key repeated at least %d scrolls", scrollsAwaited),
		func() bool {
			return len(sim.ax.recordedScrolls()) >= scrollsAwaited
		},
	)

	// Release stops the repeat. Wait for two identical samples a
	// stopSampleWindow apart. Each attempt at that costs the window, and the
	// first one may still catch a scroll dispatched before the release landed,
	// so the budget carries room for the sample that fails and the one that
	// confirms.
	sim.press("__keyup_j")

	sim.waitForWithin(
		simWaitHeadroom+2*stopSampleWindow,
		"repeat stopped after key release",
		func() bool {
			before := len(sim.ax.recordedScrolls())

			time.Sleep(stopSampleWindow)

			return len(sim.ax.recordedScrolls()) == before
		},
	)

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_HeldMotionDiagonal covers the held-key glide. Two held direction
// keys glide the cursor diagonally on the motion tick, native autorepeat of a
// held key is ignored, and releasing both keys stops the glide.
func TestSimulation_HeldMotionDiagonal(t *testing.T) {
	// stopSampleWindow is how long the stop check lets the loop run between
	// its two samples.
	const stopSampleWindow = 100 * time.Millisecond

	cfg := simConfig()
	cfg.HeldRepeat.Enabled = true

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)

	start := sim.cursor.position()

	// Right and Down are bound to relative mouse movement. Held together they
	// steer one diagonal glide. The repeated Right is what the OS autorepeat
	// delivers for a key that stays down.
	sim.press("Right")
	sim.press("Down")
	sim.press("Right")

	sim.waitFor("cursor glided down-right", func() bool {
		pos := sim.cursor.position()

		return pos.X > start.X && pos.Y > start.Y
	})

	sim.press("__keyup_Right")
	sim.press("__keyup_Down")

	sim.waitForWithin(
		simWaitHeadroom+2*stopSampleWindow,
		"glide stopped after both keys were released",
		func() bool {
			before := sim.cursor.moveCount()

			time.Sleep(stopSampleWindow)

			return sim.cursor.moveCount() == before
		},
	)

	// The two presses and the two releases each land as separate events, so
	// a tick can fire while only one key is held and move that axis alone.
	// A late tick integrates up to the controller's cap, so the bound is that
	// much travel at the base speed: under the defaults four fifths of a
	// step. Anything past that is a real drift, since an autorepeat that got
	// through moves a whole step.
	_, step, _ := cfg.HeldRepeat.HeldMotion([]string{config.CmdMoveMouseRight})
	tickShare := heldmotion.MaxTickDelta.Seconds() / cfg.HeldRepeat.Ramp().Interval.Seconds()
	strayTick := int(math.Ceil(float64(step) * tickShare))

	end := sim.cursor.position()
	if drift := (end.X - start.X) - (end.Y - start.Y); drift < -strayTick || drift > strayTick {
		t.Fatalf("diagonal drifted: start %v, end %v", start, end)
	}

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_HeldMotionDropsTheGridSelection pins that a held-key glide
// leaves the click where the cursor went, not on the region the grid had
// selected: select a region, glide away from it, click.
func TestSimulation_HeldMotionDropsTheGridSelection(t *testing.T) {
	cfg := simConfig()
	cfg.HeldRepeat.Enabled = true

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(recursiveGridHotkey)
	sim.waitMode(domain.ModeRecursiveGrid)

	sim.waitFor("recursive grid drawn", func() bool {
		_, ok := sim.overlay.lastRecursiveGridBounds()

		return ok
	})

	movesBefore := sim.cursor.moveCount()

	// "r" selects the top-left cell, which moves the cursor onto its center.
	sim.press("r")

	sim.waitFor("cursor moved onto the selected region", func() bool {
		return sim.cursor.moveCount() > movesBefore
	})

	selected := sim.cursor.position()

	sim.press("Right")

	sim.waitFor("cursor glided right", func() bool {
		return sim.cursor.position().X > selected.X
	})

	sim.press("__keyup_Right")

	sim.press("Shift+L") // left_click

	sim.waitFor("click recorded", func() bool {
		return len(sim.ax.recordedClicks()) >= 1
	})

	got := sim.ax.recordedClicks()[0]
	if got.action != action.TypeLeftClick {
		t.Fatalf("expected a left click, got %+v", got)
	}

	if got.point.Y != selected.Y || got.point.X <= selected.X {
		t.Fatalf("click at %v, expected to the right of the grid selection %v", got.point, selected)
	}

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_DragJourney covers the drag primitives: mouse down at the
// cursor, arrow-key relative movement while holding, mouse up at the new
// position.
func TestSimulation_DragJourney(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)

	start := sim.cursor.position()

	sim.press("Shift+I") // left_mouse_down

	sim.waitFor("mouse down recorded", func() bool {
		return len(sim.ax.recordedClicks()) >= 1
	})

	if got := sim.ax.recordedClicks()[0]; got.action != action.TypeLeftMouseDown ||
		got.point != start {
		t.Fatalf("expected left mouse down at %v, got %+v", start, got)
	}

	// Move while holding. Down is bound to relative mouse movement.
	sim.press("Down")

	sim.waitFor("cursor dragged downward", func() bool {
		pos := sim.cursor.position()

		return pos.Y > start.Y && pos.X == start.X
	})

	sim.press("Shift+U") // left_mouse_up

	sim.waitFor("mouse up recorded", func() bool {
		return len(sim.ax.recordedClicks()) >= 2
	})

	dragEnd := sim.cursor.position()

	if got := sim.ax.recordedClicks()[1]; got.action != action.TypeLeftMouseUp ||
		got.point != dragEnd {
		t.Fatalf("expected left mouse up at %v, got %+v", dragEnd, got)
	}

	if dragEnd == start {
		t.Fatal("drag must end at a different point than it started")
	}
}

// TestSimulation_ReleaseHeldButtonsOnExit pins the safety invariant. Exiting
// a mode between a mouse down and its release must release held buttons, or
// the desktop is left stuck mid-drag.
func TestSimulation_ReleaseHeldButtonsOnExit(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)

	sim.press("Shift+I") // left_mouse_down

	sim.waitFor("mouse down recorded", func() bool {
		return len(sim.ax.recordedClicks()) >= 1
	})

	releasesBefore := sim.ax.releaseCount()

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	sim.waitFor("held buttons released on mode exit", func() bool {
		return sim.ax.releaseCount() > releasesBefore
	})
}

// TestSimulation_RapidModeSwitching covers direct mode-to-mode transitions
// (no idle in between) across repeated cycles, the pattern of an experienced
// user hopping between modes.
func TestSimulation_RapidModeSwitching(t *testing.T) {
	sim := newSimHarness(t, simConfig(), threeButtons(t))

	for range 2 {
		sim.pressHotkey(hintsHotkey)
		sim.waitMode(domain.ModeHints)

		sim.pressHotkey(gridHotkey)
		sim.waitMode(domain.ModeGrid)

		sim.pressHotkey(scrollHotkey)
		sim.waitMode(domain.ModeScroll)

		sim.press("Escape")
		sim.waitMode(domain.ModeIdle)
	}
}

// TestSimulation_ModeSwitchNeverDropsTheKeyboard is the regression for a
// reported Linux bug. The journey is a user going from one mode straight into
// another, and what must not happen in between is Neru letting go of the
// keyboard.
//
// It was letting go. Every mode but scroll ran the full exit, which releases
// the capture, and only took it back after the next mode had queried the
// screen, built its state and drawn its overlay. On Linux that is long enough
// to type into. The user switched from scroll to recursive grid, pressed their
// grid keys, and watched them arrive in Discord as text.
//
// The whole assertion is that no disable reaches the tap between the two
// activations, because a disable *is* the window. There is no way for the
// capture to drop and the keys in it to still be Neru's.
func TestSimulation_ModeSwitchNeverDropsTheKeyboard(t *testing.T) {
	sim := newSimHarness(t, simConfig(), threeButtons(t))

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)
	sim.waitFor("the mode taking keystrokes", sim.tap.IsEnabled)

	// Record from here, because the calls made on the way into scroll are not what
	// this journey is about.
	var (
		releasedMu sync.Mutex
		released   bool
	)

	sim.tap.SetOnCall(func(label string) {
		if label != "disable" {
			return
		}

		releasedMu.Lock()
		defer releasedMu.Unlock()

		released = true
	})

	// Every mode-to-mode pair goes through the same exit, so the two that are
	// not scroll are worth walking as well, because scroll was the only one already
	// carrying the carve-out this fixes.
	for _, step := range []struct {
		hotkey string
		mode   domain.Mode
	}{
		{recursiveGridHotkey, domain.ModeRecursiveGrid},
		{gridHotkey, domain.ModeGrid},
		{hintsHotkey, domain.ModeHints},
	} {
		sim.pressHotkey(step.hotkey)
		sim.waitMode(step.mode)

		releasedMu.Lock()
		dropped := released
		releasedMu.Unlock()

		if dropped {
			t.Fatalf(
				"the keyboard was released on the way into %s: "+
					"everything typed in that window reaches the focused application instead of Neru",
				domain.ModeString(step.mode),
			)
		}

		if !sim.tap.IsEnabled() {
			t.Fatalf("%s is active with no keyboard capture", domain.ModeString(step.mode))
		}
	}
}

// TestSimulation_KeyPressedDuringAModeSwitchLandsInTheNewMode is the reporter's
// original ask, which the two tests around this one do not actually cover:
// "I would at least like it if those inputs are delayed than if they are
// ignored". Keeping the keyboard is what makes that possible, but what delivers
// it is the lock. An activation holds h.mu from start to finish and the tap's
// dispatcher delivers through HandleKeyPress, which takes the same lock, and
// nothing pinned that the key actually arrives.
//
// The switch is held open at the overlay draw, which is where a real Linux
// activation spends its time and which runs under h.mu. With the activation
// parked there, a grid key is pressed. It must not be dropped, and it must not
// be handled by the mode being left. It has to wait and land in the mode coming
// up, which is observable as the recursive grid zooming into the cell that key
// names.
//
// This pins the delivery half only, and deliberately, because press() calls
// HandleKeyPress directly, the way the tap's dispatcher does, so what is
// asserted here is that a key arriving mid-activation waits on the lock and is
// handled by the new mode. That it is *captured* rather than leaked to the
// focused application in the first place is the other half, and belongs to
// TestSimulation_ModeSwitchNeverDropsTheKeyboard above. Neither test is the
// whole claim on its own.
//
// What none of the three covers is the race that remains. A key the tap reads
// *before* the activation takes h.mu is still the old mode's, and scroll drops
// it (handleGenericScrollKey does nothing). The hotkey dispatch and the tap's
// dispatcher are separate goroutines contending for the lock unordered, so that
// window is not closed by anything here. See exitModeForTransition, which says
// so rather than claiming the whole ask is delivered.
func TestSimulation_KeyPressedDuringAModeSwitchLandsInTheNewMode(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)
	sim.waitFor("the mode taking keystrokes", sim.tap.IsEnabled)

	var (
		activationParked = make(chan struct{})
		releaseGate      = make(chan struct{})
		parkOnce         sync.Once
	)

	sim.overlay.setShowFrameGate(func(frame ports.Frame) {
		if _, isRecursiveGrid := frame.(ports.RecursiveGridFrame); !isRecursiveGrid {
			return
		}

		parkOnce.Do(func() {
			close(activationParked)
			<-releaseGate
		})
	})

	go sim.pressHotkey(recursiveGridHotkey)

	<-activationParked

	// The activation is now inside the draw, holding h.mu. This press goes to
	// the tap's side of the handler and can only be delivered once the lock is
	// free, by which time recursive grid is the active mode. "r" is the
	// top-left cell of the default 3x3 layout.
	keyDelivered := make(chan struct{})

	go func() {
		defer close(keyDelivered)

		sim.press("r")
	}()

	// Give the press long enough to be wrongly handled by scroll, where the
	// generic key handler does nothing, before letting the activation finish.
	time.Sleep(simPollInterval)
	close(releaseGate)

	sim.waitMode(domain.ModeRecursiveGrid)
	<-keyDelivered

	sim.overlay.setShowFrameGate(nil)

	topLeftThird := image.Rect(0, 0, simScreen.Dx()/3+1, simScreen.Dy()/3+1)
	sim.waitFor("the key pressed mid-switch zoomed the new mode's grid", func() bool {
		bounds, ok := sim.overlay.lastRecursiveGridBounds()

		return ok && bounds.In(topLeftThird)
	})
}

// TestSimulation_AbandonedModeSwitchGivesTheKeyboardBack is the other half of
// the journey above, and the reason keeping the capture is safe. The transition
// hands the keyboard to a mode that may never arrive.
//
// Here it does not. Hints is asked for on a screen with nothing clickable, so
// the activation gives up after the exit that kept the capture. The app is left
// idle, and idle holding the keyboard would be worse than the bug being fixed.
// Every key the user pressed would go nowhere at all.
func TestSimulation_AbandonedModeSwitchGivesTheKeyboardBack(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("the mode taking keystrokes", sim.tap.IsEnabled)

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeIdle)

	sim.waitFor("the keyboard given back", func() bool { return !sim.tap.IsEnabled() })
}

// TestSimulation_ThemeChangeReachesVisibleOverlay covers the journey the
// overlay's Style ownership exists for. Hints are on screen when the system
// switches to dark mode, and the user sees them redrawn in the new theme
// rather than left in the old one until the mode is re-entered.
//
// The overlay owns config + theme -> Style, so at this seam the appearance is
// not observable and does not need to be. What reaches the overlay is one
// notification to re-resolve, followed by a redraw of the frame already up.
// That the re-resolution actually produces the dark theme's colors is pinned
// where it happens, in TestStyleResolver_RefreshPicksUpTheNewTheme.
func TestSimulation_ThemeChangeReachesVisibleOverlay(t *testing.T) {
	sim := newSimHarness(t, simConfig(), threeButtons(t))

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	refreshesBefore := sim.overlay.styleRefreshCount()
	drawsBeforeThemeChange := sim.overlay.hintDrawCount()

	sim.switchToDarkMode()

	sim.waitFor("hints redrawn after the theme change", func() bool {
		return sim.overlay.hintDrawCount() > drawsBeforeThemeChange
	})

	if got := sim.overlay.styleRefreshCount(); got <= refreshesBefore {
		t.Errorf(
			"the overlay was asked to re-resolve its styles %d times, was %d before the theme change; it kept the old appearance",
			got,
			refreshesBefore,
		)
	}

	if !sim.overlay.isVisible() {
		t.Error("the overlay was hidden by the theme change")
	}

	if got := sim.app.CurrentMode(); got != domain.ModeHints {
		t.Errorf("mode after the theme change = %v, want hints", got)
	}
}

// TestSimulation_ConfigReloadReachesTheOverlay is the config half of the same
// ownership. A reload notifies the overlay once, with the colors the reloaded
// file asks for, and the mode drawn after it goes back on screen.
func TestSimulation_ConfigReloadReachesTheOverlay(t *testing.T) {
	sim := newSimHarness(t, simConfig(), threeButtons(t))

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	const reloadedTextColor = "#ABCDEF"

	configPath := filepath.Join(t.TempDir(), "config.toml")

	writeErr := os.WriteFile(configPath, fmt.Appendf(nil, `
[hotkeys]
%q = "hints"

[hints.ui]
text_color = %q
`, hintsHotkey, reloadedTextColor), 0o600)
	if writeErr != nil {
		t.Fatalf("failed to write the reloaded config: %v", writeErr)
	}

	reloadErr := sim.app.ReloadConfig(context.Background(), configPath)
	if reloadErr != nil {
		t.Fatalf("ReloadConfig() error = %v", reloadErr)
	}

	applied, reached := sim.overlay.lastAppliedConfig()
	if !reached {
		t.Fatal("the reload never reached the overlay")
	}

	if applied.Hints.UI.TextColor.Light != reloadedTextColor {
		t.Errorf("the overlay was handed hint text color %q, want %q",
			applied.Hints.UI.TextColor.Light, reloadedTextColor)
	}

	drawsBeforeReactivation := sim.overlay.hintDrawCount()

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints redrawn after the reload", func() bool {
		return sim.overlay.hintDrawCount() > drawsBeforeReactivation
	})
}

// TestSimulation_ConfigReloadEnablingHintsShowsHints turns hints on with a
// reload and then presses the hints hotkey. The daemon draws hints whatever
// the setting was at launch, including when an earlier reload turned them off.
func TestSimulation_ConfigReloadEnablingHintsShowsHints(t *testing.T) {
	writeConfig := func(t *testing.T, enabled bool) string {
		t.Helper()

		configPath := filepath.Join(t.TempDir(), "config.toml")

		writeErr := os.WriteFile(configPath, fmt.Appendf(nil, `
[hotkeys]
%q = "hints"

[hints]
enabled = %t
`, hintsHotkey, enabled), 0o600)
		if writeErr != nil {
			t.Fatalf("failed to write the reloaded config: %v", writeErr)
		}

		return configPath
	}

	tests := []struct {
		name            string
		enabledAtLaunch bool
		reloads         []bool
	}{
		{name: "disabled at launch", enabledAtLaunch: false, reloads: []bool{true}},
		{
			name:            "disabled by a reload, then enabled again",
			enabledAtLaunch: true,
			reloads:         []bool{false, true},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := simConfig()
			cfg.Hints.Enabled = test.enabledAtLaunch

			sim := newSimHarness(t, cfg, threeButtons(t))

			for _, enabled := range test.reloads {
				reloadErr := sim.app.ReloadConfig(context.Background(), writeConfig(t, enabled))
				if reloadErr != nil {
					t.Fatalf("ReloadConfig(enabled = %t) error = %v", enabled, reloadErr)
				}
			}

			sim.pressHotkey(hintsHotkey)
			sim.waitMode(domain.ModeHints)
			sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })
		})
	}
}

// TestSimulation_HintsNarrowingRedrawsWithoutShowingAgain pins the decision
// ADR 0003 rests on. Entering hints puts the overlay on screen once, and every
// keystroke that narrows the labels redraws it without paying for the window
// sequence again. On macOS a show queues window-level, collection-behavior
// and ordering work on the main thread. Doing that per keystroke is the
// latency regression AGENTS.md forbids. The redraw also happens before the
// keystroke returns, because a redraw left on a timer adds latency to every
// label the user types.
func TestSimulation_HintsNarrowingRedrawsWithoutShowingAgain(t *testing.T) {
	elements := manyButtons(t, 12)
	sim := newSimHarness(t, simConfig(), elements)

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	labels := sim.overlay.lastHintLabels()

	prefix := ""

	for _, candidate := range labels {
		if len(candidate) >= 2 {
			prefix = strings.ToLower(candidate[:1])

			break
		}
	}

	if prefix == "" {
		t.Fatalf("expected a multi-character label for %d elements, got %v", len(elements), labels)
	}

	showsAfterActivation := sim.overlay.showCount()
	if showsAfterActivation == 0 {
		t.Fatal("entering hints never put the overlay on screen")
	}

	drawsBefore := sim.overlay.hintDrawCount()

	sim.press(prefix)

	if got := len(sim.overlay.lastHintLabels()); got == 0 || got >= len(elements) {
		t.Fatalf("labels on screen after the narrowing key returned = %d, want between 1 and %d",
			got, len(elements)-1)
	}

	if got := sim.overlay.hintDrawCount(); got <= drawsBefore {
		t.Fatalf("hint draws after narrowing = %d, want more than %d", got, drawsBefore)
	}

	if got := sim.overlay.showCount(); got != showsAfterActivation {
		t.Errorf(
			"overlay shown %d times after narrowing, want %d: a keystroke is paying for the window sequence",
			got,
			showsAfterActivation,
		)
	}
}

// TestSimulation_ModeTransitionLeavesOneModeDrawn pins what a user sees when
// one mode replaces another. The mode they left is off the screen by the time
// the one they entered is on it, never both at once.
//
// It walks the four surfaces that draw, which are grid, hints, recursive grid
// and monitor-select, switching straight from each to the next without going
// through idle first, because a transition is where two overlays used to be
// left on screen together. The assertion is on what is drawn, not on the calls
// that got it there. Which of them the overlay uses to realize a Frame is
// exactly what this seam must be free to change.
func TestSimulation_ModeTransitionLeavesOneModeDrawn(t *testing.T) {
	cfg := monitorSelectConfig()
	cfg.Hotkeys.Bindings[recursiveGridHotkey] = []string{"recursive_grid"}

	sim := newSimHarnessWithDisplays(t, cfg, threeButtons(t), []simDisplay{
		{name: mainDisplayName, bounds: simScreen},
		{name: secondDisplayName, bounds: image.Rect(1920, 0, 3840, 1080)},
	})

	transitions := []struct {
		hotkey string
		mode   domain.Mode
	}{
		{gridHotkey, domain.ModeGrid},
		{hintsHotkey, domain.ModeHints},
		{recursiveGridHotkey, domain.ModeRecursiveGrid},
		{monitorSelectHotkey, domain.ModeMonitorSelect},
		{gridHotkey, domain.ModeGrid},
	}

	for _, transition := range transitions {
		drawn := domain.ModeString(transition.mode)

		sim.pressHotkey(transition.hotkey)
		sim.waitMode(transition.mode)

		sim.waitFor(drawn+" on screen", func() bool {
			names := sim.overlay.drawnModeNames()

			return len(names) == 1 && names[0] == drawn
		})
	}
}

// TestSimulation_LeavingMonitorSelectForScrollTakesItsPanelsDown covers the
// one transition that does not go through the common mode cleanup. Scroll is
// entered without leaving the previous mode first, so that its event tap stays
// up. The monitor picker draws on panels of its own rather than on the shared
// surface, so nothing about clearing that surface takes them off the screen,
// and a user who switched to scroll would be looking at the picker over their
// work.
func TestSimulation_LeavingMonitorSelectForScrollTakesItsPanelsDown(t *testing.T) {
	sim := newSimHarnessWithDisplays(t, monitorSelectConfig(), nil, []simDisplay{
		{name: mainDisplayName, bounds: simScreen},
		{name: secondDisplayName, bounds: image.Rect(1920, 0, 3840, 1080)},
	})

	sim.pressHotkey(monitorSelectHotkey)
	sim.waitMode(domain.ModeMonitorSelect)
	sim.waitFor("monitor panels drawn", func() bool {
		return len(sim.overlay.drawnModeNames()) == 1
	})

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	sim.waitFor("nothing left on screen", func() bool {
		return len(sim.overlay.drawnModeNames()) == 0
	})
}

// moveMonitorHotkey runs the move-monitor action rather than opening a mode.
const moveMonitorHotkey = "Primary+Shift+N"

// moveMonitorBinding is the step moveMonitorHotkey is bound to.
const moveMonitorBinding = "action move_monitor"

// TestSimulation_MonitorMoveRedrawsTheModeOnTheNewDisplay covers what a user
// sees when they send the cursor to another display with a mode open. The
// overlay leaves the display they came from and is back on the one they land
// on, with only that mode drawn.
//
// The path it exercises takes the frame off the screen before the warp and
// hands over a new one after it, across the handler lock and the monitor-move
// lock, which is why it is worth driving end to end rather than asserting on
// the calls in between.
func TestSimulation_MonitorMoveRedrawsTheModeOnTheNewDisplay(t *testing.T) {
	cfg := simConfig()
	cfg.Grid.MaxLabelLength = 2
	cfg.Hotkeys.Bindings[moveMonitorHotkey] = []string{
		"action move_monitor --name " + secondDisplayName,
	}

	second := image.Rect(1920, 0, 3840, 1080)
	sim := newSimHarnessWithDisplays(t, cfg, nil, []simDisplay{
		{name: mainDisplayName, bounds: simScreen},
		{name: secondDisplayName, bounds: second},
	})

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid on screen", func() bool {
		return sim.overlay.isVisible() && len(sim.overlay.drawnModeNames()) == 1
	})

	sim.pressHotkey(moveMonitorHotkey)

	secondCenter := image.Point{X: 2880, Y: 540}
	sim.waitFor("cursor on the second display", func() bool {
		return sim.cursor.position() == secondCenter
	})

	sim.waitFor("grid back on screen after the move", func() bool {
		names := sim.overlay.drawnModeNames()

		return sim.overlay.isVisible() &&
			len(names) == 1 &&
			names[0] == domain.ModeString(domain.ModeGrid)
	})

	if sim.app.CurrentMode() != domain.ModeGrid {
		t.Fatalf("mode after monitor move = %v, want grid", sim.app.CurrentMode())
	}

	if got := sim.overlay.lastGrid().MaxLabelLength(); got != cfg.Grid.MaxLabelLength {
		t.Errorf("grid label limit after monitor move = %d, want %d", got, cfg.Grid.MaxLabelLength)
	}
}

// TestSimulation_MonitorMoveCarriesCaptureHintsToTheNewDisplay covers hints
// with a capture strategy when the cursor is sent to another display. The
// focused window stays behind on the first display, so scanning it would find
// nothing on the second one and hints would close. The labels have to come
// back on the display the user moved to instead.
func TestSimulation_MonitorMoveCarriesCaptureHintsToTheNewDisplay(t *testing.T) {
	const contourHintsHotkey = "Primary+Shift+O"

	cfg := simConfig()
	cfg.Hotkeys.Bindings[contourHintsHotkey] = []string{"hints --strategy contour"}
	cfg.Hotkeys.Bindings[moveMonitorHotkey] = []string{
		"action move_monitor --name " + secondDisplayName,
	}

	second := image.Rect(1920, 0, 3840, 1080)
	sim := newSimHarnessWithDisplays(t, cfg, nil, []simDisplay{
		{name: mainDisplayName, bounds: simScreen},
		{name: secondDisplayName, bounds: second},
	})

	sim.desktop.focusWindow(image.Rect(100, 100, 900, 700))

	sim.pressHotkey(contourHintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn on the first display", func() bool {
		screen, drawn := sim.overlay.lastHintScreen()

		return drawn && screen == simScreen
	})

	sim.pressHotkey(moveMonitorHotkey)

	sim.waitFor("hints drawn on the second display", func() bool {
		screen, drawn := sim.overlay.lastHintScreen()

		return drawn && screen == second
	})

	if got := sim.app.CurrentMode(); got != domain.ModeHints {
		t.Fatalf("mode after the monitor move = %v, want hints", got)
	}
}

// TestSimulation_ScreenReadingRefreshLeavesOldLabelsOutOfTheCapture covers
// pressing a capture-strategy hints binding again while its labels are up. The
// refresh scans the screen with the old labels still drawn, so on a platform
// whose capture includes Neru's windows (Linux, Windows) the scan read them back
// as targets. There they now come off before the capture. Where the capture
// leaves Neru out (macOS) they stay up, as before.
func TestSimulation_ScreenReadingRefreshLeavesOldLabelsOutOfTheCapture(t *testing.T) {
	const contourHintsHotkey = "Primary+Shift+O"

	for _, testCase := range []struct {
		name               string
		excludesOwnWindows bool
		wantLabelsInFrame  bool
	}{
		{name: "capture includes Neru's windows", excludesOwnWindows: false, wantLabelsInFrame: false},
		{name: "capture leaves Neru out", excludesOwnWindows: true, wantLabelsInFrame: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := simConfig()
			cfg.Hotkeys.Bindings[contourHintsHotkey] = []string{"hints --strategy contour"}

			sim := newSimHarness(t, cfg, nil)
			sim.vision.ExcludesOwnWindows = testCase.excludesOwnWindows

			sim.desktop.focusWindow(image.Rect(100, 100, 900, 700))

			sim.pressHotkey(contourHintsHotkey)
			sim.waitMode(domain.ModeHints)
			sim.waitFor(
				"the first labels drawn",
				func() bool { return sim.overlay.hintDrawCount() >= 1 },
			)

			sim.pressHotkey(contourHintsHotkey)
			sim.waitFor(
				"the refresh scan",
				func() bool { return len(sim.vision.onScreenAtCaptures()) >= 2 },
			)

			refresh := sim.vision.onScreenAtCaptures()[1]
			if got := slices.Contains(
				refresh,
				domain.ModeNameHints,
			); got != testCase.wantLabelsInFrame {
				t.Errorf(
					"hint labels on screen during the refresh capture = %t, want %t (on screen: %v)",
					got,
					testCase.wantLabelsInFrame,
					refresh,
				)
			}

			sim.waitFor(
				"the refreshed labels drawn",
				func() bool { return sim.overlay.hintDrawCount() >= 2 },
			)
		})
	}
}

// TestSimulation_MissionControlDetectionFollowsAReload covers
// hints.detect_mission_control changing on reload and on `config set`. It used
// to be armed once at startup, so switching it on or off took a daemon restart.
// The test checks both readers: the watcher, which reports Mission Control
// opening, and the accessibility port, which aims hints at it.
func TestSimulation_MissionControlDetectionFollowsAReload(t *testing.T) {
	sim := newSimHarness(t, simConfig(), threeButtons(t))

	for _, detect := range []bool{true, false} {
		configPath := filepath.Join(t.TempDir(), "config.toml")

		writeErr := os.WriteFile(configPath, fmt.Appendf(nil, `
[hints]
include_dock_hints = true
detect_mission_control = %t

[hotkeys]
%q = "hints"
`, detect, hintsHotkey), 0o600)
		if writeErr != nil {
			t.Fatalf("failed to write the reloaded config: %v", writeErr)
		}

		reloadErr := sim.app.ReloadConfig(context.Background(), configPath)
		if reloadErr != nil {
			t.Fatalf("ReloadConfig() error = %v", reloadErr)
		}

		assertMissionControlDetection(t, sim, detect, "reloading")

		setErr := sim.app.SetConfigField(context.Background(),
			"hints.detect_mission_control", strconv.FormatBool(!detect))
		if setErr != nil {
			t.Fatalf("SetConfigField() error = %v", setErr)
		}

		assertMissionControlDetection(t, sim, !detect, "setting")
	}
}

func assertMissionControlDetection(t *testing.T, sim *simHarness, want bool, how string) {
	t.Helper()

	if got := sim.watcher.MCDetectionEnabled(); got != want {
		t.Errorf("watcher detection after %s detect_mission_control = %t is %t", how, want, got)
	}

	if got := sim.ax.missionControlDetection(); got != want {
		t.Errorf("hint scan detection after %s detect_mission_control = %t is %t", how, want, got)
	}
}

// TestSimulation_ScreenChangeWhileIdleLeavesTheOverlayHidden pins the quietest
// display change there is. Plugging a monitor in with no mode open must not
// flash an overlay at the user.
//
// Idle is the state a user is in almost all of the time, so this is the screen
// change that happens most. The overlay window coming up for it would be
// visible on every dock, undock and wake.
func TestSimulation_ScreenChangeWhileIdleLeavesTheOverlayHidden(t *testing.T) {
	sim := newSimHarness(t, simConfig(), threeButtons(t))

	if sim.app.CurrentMode() != domain.ModeIdle {
		t.Fatalf("the app started in mode %v, want idle", sim.app.CurrentMode())
	}

	showsBefore := sim.overlay.showCount()

	sim.changeScreen(simDisplayResized())

	if sim.overlay.isVisible() {
		t.Error("the screen change put the overlay on screen while the app was idle")
	}

	if drawn := sim.overlay.drawnModeNames(); len(drawn) != 0 {
		t.Errorf("the screen change drew %v while the app was idle, want nothing", drawn)
	}

	if got := sim.overlay.showCount(); got != showsBefore {
		t.Errorf("overlay shows = %d after an idle screen change, want %d", got, showsBefore)
	}

	if got := sim.app.CurrentMode(); got != domain.ModeIdle {
		t.Errorf("mode after an idle screen change = %v, want idle", got)
	}
}

// TestSimulation_ScreenChangeRegeneratesHintsOnTheNewScreen covers the
// heaviest thing a display change does. Hints are open when the arrangement
// changes, and the labels a user is looking at have to come from the
// accessibility tree as it is now, drawn against the display as it is now.
//
// Stale labels here are worse than none. They point at positions that no
// longer exist, so typing one moves the cursor somewhere the user did not
// choose. The journey therefore changes the tree along with the display and
// asserts the drawn labels followed it.
func TestSimulation_ScreenChangeRegeneratesHintsOnTheNewScreen(t *testing.T) {
	sim := newSimHarness(t, simConfig(), threeButtons(t))

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	if got := len(sim.overlay.lastHintLabels()); got != 3 {
		t.Fatalf("hints drawn for %d elements before the screen change, want 3", got)
	}

	drawsBefore := sim.overlay.hintDrawCount()
	cursorBefore := sim.cursor.position()
	movesBefore := sim.cursor.moveCount()

	// The display change relaid out the windows. One of the three buttons is
	// gone from the tree.
	sim.ax.setElements([]*element.Element{
		simElement(t, "save", image.Rect(100, 100, 220, 140), "Save"),
		simElement(t, "cancel", image.Rect(300, 100, 420, 140), "Cancel"),
	})

	sim.changeScreen(simDisplayResized())

	sim.waitFor("hints redrawn after the screen change", func() bool {
		return sim.overlay.hintDrawCount() > drawsBefore
	})

	if got := len(sim.overlay.lastHintLabels()); got != 2 {
		t.Errorf(
			"hints drawn for %d elements after the screen change, want 2: the collection was not regenerated",
			got,
		)
	}

	screen, drawn := sim.overlay.lastHintScreen()
	if !drawn || screen != simScreenResized {
		t.Errorf("hints drawn against screen %v (drawn = %v), want %v",
			screen, drawn, simScreenResized)
	}

	if !sim.overlay.isVisible() {
		t.Error("the screen change left the hints overlay off the screen")
	}

	// The user chose none of this. A display changing under them must leave
	// their pointer where they put it.
	if got := sim.cursor.moveCount(); got != movesBefore {
		t.Errorf(
			"the cursor was moved %d times by the screen change, from %v to %v; it must stay where the user left it",
			got-movesBefore,
			cursorBefore,
			sim.cursor.position(),
		)
	}

	if got := sim.app.CurrentMode(); got != domain.ModeHints {
		t.Errorf("mode after the screen change = %v, want hints", got)
	}
}

// TestSimulation_ScreenChangeRebuildsGridAndClearsSelection covers the grid
// half. The cells a user is about to type have to describe the display as it
// now is, and the selection they made on the display that is gone has to go
// with it, because a cell coordinate from the old layout selects a different point on
// the new one.
//
// The selection is read off the pointer the grid draws for it, so the journey
// turns cursor-follow off first ("`"), because with the cursor following, the selection
// has nothing on screen of its own.
func TestSimulation_ScreenChangeRebuildsGridAndClearsSelection(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	if got, want := sim.overlay.lastGrid().Bounds(), localBounds(simScreen); got != want {
		t.Fatalf("grid drawn over %v before the screen change, want %v", got, want)
	}

	sim.press("`") // stop the cursor following the selection

	cells := sim.overlay.lastGrid().Cells()
	if len(cells) == 0 {
		t.Fatal("grid drawn with zero cells")
	}

	sim.typeLabel(cells[len(cells)/2].Coordinate())

	sim.waitFor("the selection is on screen", func() bool {
		pointer, drawn := sim.overlay.lastGridPointer()

		return drawn && pointer.Visible
	})

	drawsBefore := sim.overlay.gridDrawCount()

	sim.changeScreen(simDisplayResized())

	sim.waitFor("grid redrawn after the screen change", func() bool {
		return sim.overlay.gridDrawCount() > drawsBefore
	})

	if got, want := sim.overlay.lastGrid().Bounds(), localBounds(simScreenResized); got != want {
		t.Errorf("grid drawn over %v after the screen change, want %v: it was not rebuilt",
			got, want)
	}

	if pointer, _ := sim.overlay.lastGridPointer(); pointer.Visible {
		t.Errorf(
			"the selection at %v survived the screen change; it points at a place on a display that is gone",
			pointer.Position,
		)
	}

	if !sim.overlay.isVisible() {
		t.Error("the screen change left the grid off the screen")
	}

	if got := sim.app.CurrentMode(); got != domain.ModeGrid {
		t.Errorf("mode after the screen change = %v, want grid", got)
	}
}

// TestSimulation_ScreenChangePreservesTheZoomedRegion covers what a display
// change must not cost a recursive-grid user: the region they have already
// zoomed into. Recursive grid is a sequence of narrowing choices, so throwing
// the region away would throw their progress away with it. The mode is
// remapped onto the new display rather than restarted on it.
//
// "The same region" means the same fraction of the display, which is what the
// journey asserts. The zoomed rectangle covers the same proportion of the
// smaller screen as it did of the larger one.
func TestSimulation_ScreenChangePreservesTheZoomedRegion(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(recursiveGridHotkey)
	sim.waitMode(domain.ModeRecursiveGrid)
	sim.waitFor("recursive grid drawn", func() bool {
		_, ok := sim.overlay.lastRecursiveGridBounds()

		return ok
	})

	// "r" is the top-left cell of the default 3x3 key layout.
	sim.press("r")

	topLeftThird := image.Rect(0, 0, simScreen.Dx()/3+1, simScreen.Dy()/3+1)
	sim.waitFor("grid zoomed into the top-left cell", func() bool {
		bounds, ok := sim.overlay.lastRecursiveGridBounds()

		return ok && bounds.In(topLeftThird)
	})

	zoomed, _ := sim.overlay.lastRecursiveGridBounds()
	drawsBefore := sim.overlay.recursiveGridDrawCount()

	sim.changeScreen(simDisplayResized())

	sim.waitFor("recursive grid redrawn after the screen change", func() bool {
		return sim.overlay.recursiveGridDrawCount() > drawsBefore
	})

	remapped, drawn := sim.overlay.lastRecursiveGridBounds()
	if !drawn {
		t.Fatal("the recursive grid was never drawn")
	}

	if remapped.Empty() {
		t.Fatalf("the zoomed region collapsed to %v; there is nothing left to pick from", remapped)
	}

	// The same fraction of the display, on both axes, to within a pixel of
	// rounding.
	expected := image.Rect(
		scaleAxis(zoomed.Min.X, simScreen.Dx(), simScreenResized.Dx()),
		scaleAxis(zoomed.Min.Y, simScreen.Dy(), simScreenResized.Dy()),
		scaleAxis(zoomed.Max.X, simScreen.Dx(), simScreenResized.Dx()),
		scaleAxis(zoomed.Max.Y, simScreen.Dy(), simScreenResized.Dy()),
	)

	if !boundsWithin(remapped, expected, 1) {
		t.Errorf(
			"the region zoomed to %v on a %v screen came back as %v on a %v one, want about %v",
			zoomed, localBounds(simScreen), remapped, localBounds(simScreenResized), expected,
		)
	}

	if !sim.overlay.isVisible() {
		t.Error("the screen change left the recursive grid off the screen")
	}

	if got := sim.app.CurrentMode(); got != domain.ModeRecursiveGrid {
		t.Errorf("mode after the screen change = %v, want recursive grid", got)
	}
}

// TestSimulation_ScreenChangeWithNothingToDrawExitsTheMode pins the failure
// path. The display changed, hints were regenerated for it, and this time the
// accessibility tree has nothing to offer. The window the labels belonged to
// went with the display.
//
// Leaving the mode running would strand the user in hints with the old
// display's labels on screen, so the mode is left instead and the overlay comes
// down. Idle with nothing drawn is recoverable, but a mode showing labels that
// point nowhere is not.
func TestSimulation_ScreenChangeWithNothingToDrawExitsTheMode(t *testing.T) {
	sim := newSimHarness(t, simConfig(), threeButtons(t))

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints on screen", func() bool {
		return sim.overlay.hintDrawCount() > 0 && sim.overlay.isVisible()
	})

	// The display change took the window the hints belonged to with it.
	sim.ax.setElements(nil)

	sim.changeScreen(simDisplayResized())

	sim.waitMode(domain.ModeIdle)

	sim.waitFor("the overlay came down with the mode", func() bool {
		return !sim.overlay.isVisible() && len(sim.overlay.drawnModeNames()) == 0
	})

	// The app is still usable afterwards. A mode that needs no elements still
	// activates.
	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
}

// The display-change journeys above all have the mode open when the
// arrangement changes. The four below are the other half of that set. The
// change lands while the mode is *inactive*, and the mode has to come back
// built for the display that exists now rather than the one it last drew on.
//
// Nothing records that a display changed while a mode was away, because each
// activation reads the current arrangement unconditionally, so what these
// pin is that eager rebuild. It is the whole mechanism protecting a user who
// docks, undocks or changes resolution between two activations, and without a
// journey on it, replacing it with anything cheaper would look free.
//
// Four rather than five, because scroll draws no surface of its own, so it has no
// frame built for a display for a journey to read. Its overlay content is the
// indicators, which are placed by the adapter rather than described in a
// Frame.

// TestSimulation_GridComesBackOnTheDisplayThatChangedWhileItWasInactive covers
// the commonest shape of this. A user leaves grid mode, plugs a display in or
// changes resolution, and comes back to grid.
//
// A grid built for the display that is gone is worse than no grid. Its cells
// still have labels, so typing one lands the cursor at a position derived from
// a screen the user is no longer looking at.
func TestSimulation_GridComesBackOnTheDisplayThatChangedWhileItWasInactive(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	if got, want := sim.overlay.lastGrid().Bounds(), localBounds(simScreen); got != want {
		t.Fatalf("grid drawn over %v before the display change, want %v", got, want)
	}

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	drawsBefore := sim.overlay.gridDrawCount()

	sim.changeScreen(simDisplayResized())

	// The mode is not on screen, so the change itself must draw nothing.
	// That half is TestSimulation_ScreenChangeWhileIdleLeavesTheOverlayHidden.
	// What matters here is that the rebuild happens on activation instead.
	if got := sim.overlay.gridDrawCount(); got != drawsBefore {
		t.Errorf("the grid was drawn %d times by a display change it was not open for",
			got-drawsBefore)
	}

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn again", func() bool {
		return sim.overlay.gridDrawCount() > drawsBefore
	})

	if got, want := sim.overlay.lastGrid().Bounds(), localBounds(simScreenResized); got != want {
		t.Errorf(
			"the grid came back over %v, want %v: it was built for the display that is gone",
			got, want,
		)
	}

	if !sim.overlay.isVisible() {
		t.Error("the grid was rebuilt but never put on screen")
	}
}

// TestSimulation_HintsComeBackOnTheDisplayThatChangedWhileTheyWereInactive is
// the same journey for hints, which has two things to get right rather than
// one: the display the labels are drawn against, and the accessibility tree
// they came from. A display change relays windows out, so the elements are
// not the ones the last activation saw either.
func TestSimulation_HintsComeBackOnTheDisplayThatChangedWhileTheyWereInactive(t *testing.T) {
	sim := newSimHarness(t, simConfig(), threeButtons(t))

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	if got := len(sim.overlay.lastHintLabels()); got != 3 {
		t.Fatalf("hints drawn for %d elements before the display change, want 3", got)
	}

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	drawsBefore := sim.overlay.hintDrawCount()

	// The display change relaid out the windows. One of the three buttons is
	// gone from the tree.
	sim.ax.setElements([]*element.Element{
		simElement(t, "save", image.Rect(100, 100, 220, 140), "Save"),
		simElement(t, "cancel", image.Rect(300, 100, 420, 140), "Cancel"),
	})

	sim.changeScreen(simDisplayResized())

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn again", func() bool {
		return sim.overlay.hintDrawCount() > drawsBefore
	})

	if got := len(sim.overlay.lastHintLabels()); got != 2 {
		t.Errorf(
			"hints came back with %d labels, want 2: the collection was not regenerated",
			got,
		)
	}

	screen, drawn := sim.overlay.lastHintScreen()
	if !drawn || screen != simScreenResized {
		t.Errorf("hints came back drawn against screen %v (drawn = %v), want %v",
			screen, drawn, simScreenResized)
	}

	if !sim.overlay.isVisible() {
		t.Error("hints were regenerated but never put on screen")
	}
}

// TestSimulation_RecursiveGridComesBackOnTheDisplayThatChangedWhileItWasInactive
// covers the mode with state worth stranding: a zoomed region. Left open
// across a display change the region is remapped and kept
// (TestSimulation_ScreenChangePreservesTheZoomedRegion), but an activation is
// a fresh start, so this asserts the opposite outcome, the whole of the new
// display, not the old display's zoom drawn onto it.
func TestSimulation_RecursiveGridComesBackOnTheDisplayThatChangedWhileItWasInactive(
	t *testing.T,
) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(recursiveGridHotkey)
	sim.waitMode(domain.ModeRecursiveGrid)
	sim.waitFor("recursive grid drawn", func() bool {
		_, ok := sim.overlay.lastRecursiveGridBounds()

		return ok
	})

	// "r" is the top-left cell of the default 3x3 key layout. The user got as
	// far as one narrowing choice before leaving.
	sim.press("r")

	topLeftThird := image.Rect(0, 0, simScreen.Dx()/3+1, simScreen.Dy()/3+1)
	sim.waitFor("grid zoomed into the top-left cell", func() bool {
		bounds, ok := sim.overlay.lastRecursiveGridBounds()

		return ok && bounds.In(topLeftThird)
	})

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	drawsBefore := sim.overlay.recursiveGridDrawCount()

	sim.changeScreen(simDisplayResized())

	sim.pressHotkey(recursiveGridHotkey)
	sim.waitMode(domain.ModeRecursiveGrid)
	sim.waitFor("recursive grid drawn again", func() bool {
		return sim.overlay.recursiveGridDrawCount() > drawsBefore
	})

	bounds, drawn := sim.overlay.lastRecursiveGridBounds()
	if !drawn {
		t.Fatal("the recursive grid was never drawn")
	}

	if want := localBounds(simScreenResized); bounds != want {
		t.Errorf(
			"the recursive grid came back over %v, want %v: the whole of the display as it now is",
			bounds, want,
		)
	}

	if !sim.overlay.isVisible() {
		t.Error("the recursive grid was rebuilt but never put on screen")
	}
}

// TestSimulation_MonitorSelectComesBackOnTheDisplaysThatChangedWhileItWasInactive
// closes the set with the mode whose whole content is the display
// arrangement. The picker's panels are the monitors, so a rearrangement while
// it is closed is a rearrangement of everything it draws.
//
// A panel describing a display that has been reconfigured is a target the user
// can pick and be moved to the wrong place by, which is the failure this rules
// out.
//
// Unlike its three siblings this asserts nothing about the shared overlay
// being visible, and deliberately. The picker draws on panels of its own and
// never brings that window up, so pinning it visible here would pin something
// the adapter does not do.
func TestSimulation_MonitorSelectComesBackOnTheDisplaysThatChangedWhileItWasInactive(
	t *testing.T,
) {
	second := image.Rect(1920, 0, 3840, 1080)
	sim := newSimHarnessWithDisplays(t, monitorSelectConfig(), nil, []simDisplay{
		{name: mainDisplayName, bounds: simScreen},
		{name: secondDisplayName, bounds: second},
	})

	sim.pressHotkey(monitorSelectHotkey)
	sim.waitMode(domain.ModeMonitorSelect)
	sim.waitFor("monitor panels drawn", func() bool {
		return len(sim.overlay.lastMonitorTargets()) == 2
	})

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	drawsBefore := sim.overlay.monitorDrawCount()

	// The second display came back at a different resolution while the picker
	// was closed.
	rearranged := image.Rect(1920, 0, 3200, 900)
	sim.changeScreen(
		simDisplay{name: mainDisplayName, bounds: simScreen},
		simDisplay{name: secondDisplayName, bounds: rearranged},
	)

	sim.pressHotkey(monitorSelectHotkey)
	sim.waitMode(domain.ModeMonitorSelect)
	sim.waitFor("monitor panels drawn again", func() bool {
		return sim.overlay.monitorDrawCount() > drawsBefore
	})

	targets := sim.overlay.lastMonitorTargets()
	if len(targets) != 2 {
		t.Fatalf("the picker came back with %d panels, want 2", len(targets))
	}

	// Both displays, named individually, because a count of two says nothing about
	// which two, and the picker coming back with the rearranged display twice
	// is exactly the shape a rebuild that half-followed the change would take.
	drawnFor := make(map[image.Rectangle]bool, len(targets))
	for _, target := range targets {
		drawnFor[target.Bounds] = true
	}

	for _, want := range []image.Rectangle{simScreen, rearranged} {
		if !drawnFor[want] {
			t.Errorf("no panel for the display at %v in %v", want, targets)
		}
	}

	if drawnFor[second] {
		t.Errorf(
			"the picker came back with a panel for %v, the display's bounds before the change",
			second,
		)
	}
}

// TestSimulation_ThemeChangeRedrawsMonitorSelect closes the theme-change set.
// Every other mode that draws is already pinned redrawing itself when the
// system switches between light and dark, and the monitor picker is the one
// that was not.
//
// It draws on panels of its own rather than the shared surface, so nothing
// about the other modes picking the new theme up brings it along. Left out, it
// is the one overlay a user finds still in the old theme.
//
// That the overlay re-resolves every Style for the change is not mode-specific
// and is pinned once, in TestSimulation_ThemeChangeReachesVisibleOverlay. What
// only the picker can answer for is being drawn again afterwards, which is what
// this asserts.
func TestSimulation_ThemeChangeRedrawsMonitorSelect(t *testing.T) {
	sim := newSimHarnessWithDisplays(t, monitorSelectConfig(), nil, []simDisplay{
		{name: mainDisplayName, bounds: simScreen},
		{name: secondDisplayName, bounds: image.Rect(1920, 0, 3840, 1080)},
	})

	sim.pressHotkey(monitorSelectHotkey)
	sim.waitMode(domain.ModeMonitorSelect)
	sim.waitFor("monitor panels drawn", func() bool {
		return len(sim.overlay.lastMonitorTargets()) == 2
	})

	drawsBefore := sim.overlay.monitorDrawCount()

	sim.switchToDarkMode()

	sim.waitFor("the monitor picker redrawn after the theme change", func() bool {
		return sim.overlay.monitorDrawCount() > drawsBefore
	})

	if got := len(sim.overlay.lastMonitorTargets()); got != 2 {
		t.Errorf("the picker came back with %d panels, want 2", got)
	}

	if got := sim.app.CurrentMode(); got != domain.ModeMonitorSelect {
		t.Errorf("mode after the theme change = %v, want monitor select", got)
	}
}

// TestSimulation_KeystrokeAsksWhichAppIsFocused pins what one keystroke inside
// a mode costs in questions to the operating system: none, whether or not the
// active mode declares per-app hotkey overrides.
//
// It used to be one question per keystroke where overrides were declared, and
// ADR 0005 is the decision that drove it to zero. The focused app is published
// by the application watcher and the keymap is settled when it changes, so a
// keystroke consults bindings that are already resolved.
//
// Asking which application is focused was the only thing on the keystroke path
// that left the process. On macOS it is a message to another application,
// which can be busy or wedged, and the handler holds the lock that serializes
// key handling, mode exit included, while it waits. The count is therefore
// what a user can be stalled by, which is why it is asserted here rather than
// described somewhere.
//
// Both directions are kept because they fail differently. The first is the
// regression this fixed, and the second is the mode that never paid, which a
// settle that asked unconditionally would newly charge.
//
// Grid mode drives both directions because it needs no accessibility tree of
// its own. Nothing else in the keystroke being measured has any reason to ask
// which app is focused, so the count belongs to key dispatch and to nothing
// else.
func TestSimulation_KeystrokeAsksWhichAppIsFocused(t *testing.T) {
	tests := []struct {
		name string
		cfg  func() *config.Config
		want int
	}{
		{
			name: "mode declares per-app overrides",
			cfg: func() *config.Config {
				cfg := simConfig()
				cfg.Grid.AppConfigs = []config.AppConfig{
					perAppHotkeyOverride(
						simFixtureBundleID,
						unpressedOverrideKey,
						"action left_click",
					),
				}

				return cfg
			},
			want: 0,
		},
		{
			name: "mode declares none",
			cfg:  simConfig,
			want: 0,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			sim := newSimHarness(t, testCase.cfg(), nil)

			sim.pressHotkey(gridHotkey)
			sim.waitMode(domain.ModeGrid)
			sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

			// Entering the mode is allowed to ask. That is the one-shot the
			// keymap settles on. The grid is drawn before the mode starts
			// taking keys, so this waits for the event tap as well, because the count
			// has to start from an activation that has finished, not one still
			// in flight.
			sim.waitFor("the mode taking keystrokes", sim.tap.IsEnabled)

			queriesBefore := sim.ax.focusedAppQueryCount()

			sim.press(firstGridLabelKey(sim))

			sim.waitFor("grid narrowed to what was typed", func() bool {
				prefix, narrowed := sim.overlay.lastMatchPrefix()

				return narrowed && prefix != ""
			})

			if got := sim.ax.focusedAppQueryCount() - queriesBefore; got != testCase.want {
				t.Errorf(
					"one keystroke asked which app is focused %d times, want %d",
					got, testCase.want,
				)
			}
		})
	}
}

// TestSimulation_TheAppHearsFocusChangesFromTheMomentItWatchesForThem is the
// precondition the two journeys below it rest on, and #1348 is what happens
// without it. The daemon started the application watcher and made the hotkeys
// live before it registered the callback activations are delivered to, so an
// application switch in that window reached nobody, was never retried, and left
// the journey below entering the mode the *previous* application bound the key
// to. ADR 0005 carries why a dropped publication is unrecoverable rather than
// merely late.
//
// Worth stating what it is not, because the intermittency points elsewhere and
// #1348 named two suspects. Neither goroutine `handleAppActivation` starts is
// involved. Publication is a lock-free cell write on the caller's goroutine, so
// it is already in place before the keystroke that reads it. The passthrough
// refresh only reads. And the hotkey refresh is deferred outright while a mode
// is open. What was intermittent was whether the daemon had finished starting,
// not what it did once it had.
//
// It asserts the order rather than the symptom because the order is what makes
// the symptom impossible. Whether the window is wide enough to fall into is a
// property of the machine, and CI found it on a busy runner.
func TestSimulation_TheAppHearsFocusChangesFromTheMomentItWatchesForThem(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	// The harness already waits for this, and asking again is what keeps the
	// assertion below from passing vacuously if that ever stops being true.
	registered, started := sim.watcher.ActivateCallbacksAtStart()
	if !started {
		t.Fatal("the app watcher was never started")
	}

	if registered == 0 {
		t.Error(
			"the app watcher was started before anything registered for activations: " +
				"an application switch reported in that window is dropped and never " +
				"retried, so per-app hotkey overrides go on binding to the application " +
				"the mode was opened in",
		)
	}
}

// TestSimulation_AppSwitchStopsWarmingThePreviousApp pins that switching
// applications stops the tree warmup the previous switch started. Each warmup
// walks the application's accessibility tree and keeps every element it has
// queued until it returns. Warmups that outlive their switch pile up, so a user
// who switches applications all day grows the daemon without bound.
func TestSimulation_AppSwitchStopsWarmingThePreviousApp(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	// err is the warmup context's Err, so it reports whether the warmup stopped.
	type warmup struct {
		bundleID string
		err      func() error
	}

	started := make(chan warmup, 8)
	sim.ax.setPrime(func(ctx context.Context, bundleID string) (bool, error) {
		started <- warmup{bundleID, ctx.Err}

		<-ctx.Done()

		return false, ctx.Err()
	})

	waitWarmup := func(bundleID string) func() error {
		t.Helper()

		timeout := time.After(simWaitHeadroom)
		for {
			select {
			case w := <-started:
				if w.bundleID == bundleID {
					return w.err
				}
			case <-timeout:
				t.Fatalf("no tree warmup started for %s", bundleID)
			}
		}
	}

	sim.focusApp(simOtherAppName, simOtherAppBundleID)
	previous := waitWarmup(simOtherAppBundleID)

	sim.focusApp("Fixture", simFixtureBundleID)
	current := waitWarmup(simFixtureBundleID)

	sim.waitFor("the previous app's warmup to stop", func() bool {
		return previous() != nil
	})

	if current() != nil {
		t.Error("switching applications stopped the warmup of the application now in front")
	}
}

// TestSimulation_FocusChangeMidModeRebindsTheKey covers switching applications
// without leaving the mode, which ordinarily happens by passing a shortcut
// through to the system and landing somewhere else, and pins what the
// next keystroke does. It runs the binding the newly focused application's
// overrides put on that key, not the one the mode was opened under.
//
// That is what per-app bindings mean, and it is the behavior most at risk
// from anything that decides the keymap once and keeps it. The journey drives
// the change the way the platform does, through a watcher activation event and
// a desktop that now answers with the other application, so it stays true of
// whichever half the handler learns from.
func TestSimulation_FocusChangeMidModeRebindsTheKey(t *testing.T) {
	const sharedKey = "j"

	// The two applications bind the same key to a different mode, so which
	// mode the user lands in says which application's overrides were in force.
	cfg := simConfig()
	cfg.Grid.AppConfigs = []config.AppConfig{
		perAppHotkeyOverride(simFixtureBundleID, sharedKey, config.ModeNameScroll),
		perAppHotkeyOverride(simOtherAppBundleID, sharedKey, config.ModeNameRecursiveGrid),
	}

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	sim.focusApp(simOtherAppName, simOtherAppBundleID)

	sim.press(sharedKey)

	// A mode-to-mode switch runs off the keystroke on a goroutine and passes
	// through idle on its way in, so idle is a moment in the transition rather
	// than an answer. Wait for whichever mode the binding opens.
	sim.waitFor("a binding for the shared key to open a mode", func() bool {
		mode := sim.app.CurrentMode()

		return mode != domain.ModeGrid && mode != domain.ModeIdle
	})

	if got := sim.app.CurrentMode(); got != domain.ModeRecursiveGrid {
		t.Fatalf(
			"pressing %q after switching to %s entered %s, want %s: the key still means what "+
				"it meant in the application the mode was opened in",
			sharedKey,
			simOtherAppName,
			domain.ModeString(got),
			domain.ModeString(domain.ModeRecursiveGrid),
		)
	}
}

// TestSimulation_FocusChangeMidModeRetargetsThePassthroughBlacklist is the
// other half of the journey above, and the one #1252 reported. Knowing what
// the next key means is worth nothing if the key never arrives.
//
// With passthrough on, the event tap passes every modifier chord to the
// focused application except the ones it is told to keep. Those are the chords
// the mode binds, which per-app overrides make a function of the focused
// application, so switching applications mid-mode has to move them. Otherwise
// the mode consumes what the application the user left had bound and hands the
// application they arrived in the chord it binds itself.
func TestSimulation_FocusChangeMidModeRetargetsThePassthroughBlacklist(t *testing.T) {
	const (
		fixtureChord = "Cmd+Ctrl+G"
		otherChord   = "Cmd+Ctrl+H"
	)

	canonical := config.CanonicalHotkeyForPlatform

	cfg := simConfig()
	cfg.General.PassthroughUnboundedKeys = true
	cfg.Grid.AppConfigs = []config.AppConfig{
		perAppHotkeyOverride(simFixtureBundleID, fixtureChord, config.ModeNameScroll),
		perAppHotkeyOverride(simOtherAppBundleID, otherChord, config.ModeNameRecursiveGrid),
	}

	sim := newSimHarness(t, cfg, nil)

	sim.focusApp(simFixtureAppName, simFixtureBundleID)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)

	sim.waitFor("the fixture application's chord to be intercepted", func() bool {
		return slices.Contains(sim.tap.InterceptedModifierKeys(), canonical(fixtureChord))
	})

	sim.focusApp(simOtherAppName, simOtherAppBundleID)

	sim.waitFor("the newly focused application's chord to be intercepted", func() bool {
		return slices.Contains(sim.tap.InterceptedModifierKeys(), canonical(otherChord))
	})

	if got := sim.tap.InterceptedModifierKeys(); slices.Contains(got, canonical(fixtureChord)) {
		t.Errorf(
			"the intercepted chords are %v after switching to %s, want %q gone: "+
				"nothing binds it there and the mode still swallows it",
			got, simOtherAppName, canonical(fixtureChord),
		)
	}

	_, blacklist := sim.tap.ModifierPassthrough()
	if !slices.Contains(blacklist, canonical(otherChord)) {
		t.Errorf(
			"the passthrough blacklist is %v after switching to %s, want %q among them: "+
				"the chord it binds is passed to it instead of reaching the mode",
			blacklist, simOtherAppName, canonical(otherChord),
		)
	}
}

// localBounds is a display in the overlay's screen-local space, which is what
// the grid surfaces are drawn in.
func localBounds(screen image.Rectangle) image.Rectangle {
	return image.Rect(0, 0, screen.Dx(), screen.Dy())
}

// scaleAxis maps a coordinate from a display of size from to one of size to.
func scaleAxis(value, from, to int) int {
	return value * to / from
}

// boundsWithin reports whether every edge of got is within tolerance pixels of
// the same edge of want.
func boundsWithin(got, want image.Rectangle, tolerance int) bool {
	within := func(a, b int) bool { return a-b <= tolerance && b-a <= tolerance }

	return within(got.Min.X, want.Min.X) && within(got.Min.Y, want.Min.Y) &&
		within(got.Max.X, want.Max.X) && within(got.Max.Y, want.Max.Y)
}

// TestSimulation_ConfigSetRelabelsTheGridWithoutAReload is the user-visible end
// of issue #1268. grid.row_labels means "infer from the characters the grid is
// drawn with" while it is empty, and a load settles it. After that a settled
// label is indistinguishable from one somebody typed. Applying a change to
// grid.characters on the settled configuration therefore found the labels
// already filled in and left them, so the grid went on drawing coordinates from
// a character set it no longer used until the next reload.
//
// It asserts the labels a user can actually see and press, not the config field
// they came from: the drawn cells, through the same overlay a monitor would.
func TestSimulation_ConfigSetRelabelsTheGridWithoutAReload(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	setErr := sim.app.SetConfigField(context.Background(), "grid.characters", "asdf")
	if setErr != nil {
		t.Fatalf("config set grid.characters failed: %v", setErr)
	}

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	grid := sim.overlay.lastGrid()

	if got := grid.RowLabels(); got != "ASDF" {
		t.Errorf("drawn grid RowLabels() = %q, want %q", got, "ASDF")
	}

	if got := grid.ColLabels(); got != "ASDF" {
		t.Errorf("drawn grid ColLabels() = %q, want %q", got, "ASDF")
	}

	cells := grid.Cells()
	if len(cells) == 0 {
		t.Fatal("grid drawn with zero cells")
	}

	// The labels the user reads, not just the strings they were built from, because a
	// coordinate outside the new characters is a cell nobody can type.
	for _, cell := range cells {
		if strings.ContainsFunc(cell.Coordinate(), func(r rune) bool {
			return !strings.ContainsRune("ASDF", r)
		}) {
			t.Fatalf("cell labeled %q, which the new characters cannot spell", cell.Coordinate())
		}
	}
}

// The two halves of the reported --toggle case: a global chord bound to the
// toggle, and the scroll-mode key that hands over to recursive grid.
const (
	globalToggleChord    = "Ctrl+Alt+;"
	scrollToRecursiveKey = "Space"
)

// TestSimulation_GlobalHotkeyTogglesAModeEnteredFromAnotherMode is the journey
// behind #1519's neighbor, where --toggle exits the mode it names whenever that mode
// is the active one, and how the user got there is not part of the question.
//
// It presses the chord the way the in-mode capture delivers it rather than
// through the hotkey port, because that is the only way it arrives on Linux. The
// evdev grab silences the listener that registered it, so the handler resolving
// the global table itself is what makes the toggle work at all
// (internal/app/modes/keymap.go, settledKeymaps).
func TestSimulation_GlobalHotkeyTogglesAModeEnteredFromAnotherMode(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[globalToggleChord] = []string{"recursive_grid --toggle"}
	cfg.Scroll.Hotkeys[scrollToRecursiveKey] = config.StringOrStringArray{
		config.ModeNameRecursiveGrid,
	}

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)
	sim.waitFor("the mode taking keystrokes", sim.tap.IsEnabled)

	// Into recursive grid the way the report describes it: from another mode,
	// through that mode's own binding, never through the global chord.
	sim.press(scrollToRecursiveKey)
	sim.waitMode(domain.ModeRecursiveGrid)

	sim.press(globalToggleChord)

	sim.waitFor("the global chord toggling the mode it named back to idle", func() bool {
		return sim.app.CurrentMode() == domain.ModeIdle
	})
}

// The same chord from the same mode entered the other way. Both paths answer the
// toggle, which is the property the report found broken on one of them.
func TestSimulation_GlobalHotkeyTogglesAModeItEnteredItself(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[globalToggleChord] = []string{"recursive_grid --toggle"}

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(recursiveGridHotkey)
	sim.waitMode(domain.ModeRecursiveGrid)
	sim.waitFor("the mode taking keystrokes", sim.tap.IsEnabled)

	sim.press(globalToggleChord)

	sim.waitFor("the global chord toggling the mode it opened back to idle", func() bool {
		return sim.app.CurrentMode() == domain.ModeIdle
	})
}

// TestSimulation_ModeHotkeyMatchesWhateverModifierOrderTheTapSpells pins #1733.
// A per-mode binding written "Ctrl+Shift+X" never fired on Linux, because the
// evdev capture spells the same chord "Shift+Ctrl+x". The binding must match
// whatever modifier order the tap uses.
func TestSimulation_ModeHotkeyMatchesWhateverModifierOrderTheTapSpells(t *testing.T) {
	cfg := simConfig()
	cfg.RecursiveGrid.Hotkeys["Ctrl+Shift+X"] = config.StringOrStringArray{config.ModeNameScroll}

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(recursiveGridHotkey)
	sim.waitMode(domain.ModeRecursiveGrid)
	sim.waitFor("the mode taking keystrokes", sim.tap.IsEnabled)

	sim.press("Shift+Ctrl+x")
	sim.waitMode(domain.ModeScroll)
}

// TestSimulation_StopPausesEverything pins what `neru stop` promises. The open
// mode closes, no route enters a mode until `neru start`, and the global
// hotkeys are unregistered and restored at once rather than at the next
// application switch.
//
// The journey drives stop and start through App.SetEnabled, which the IPC
// handler and the systray toggle both call. The systray's mode items call
// ActivateMode directly, so the journey tries that route as well.
func TestSimulation_StopPausesEverything(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	sim.app.SetEnabled(false)

	sim.waitMode(domain.ModeIdle)
	sim.waitFor("hotkeys unregistered", func() bool {
		return sim.hotkeys.callbackFor(config.CanonicalHotkeyForPlatform(scrollHotkey)) == nil
	})

	sim.app.ActivateMode(domain.ModeScroll)
	sim.neverMode(domain.ModeScroll, 250*time.Millisecond)

	sim.app.SetEnabled(true)

	// pressHotkey waits for the chord to be registered again. No focus change
	// happens in between, so the registration has to come from start itself.
	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// simWords makes the screen read vision returns: one line per word.
func simWords(t *testing.T, sim *simHarness, words map[string]image.Rectangle) {
	t.Helper()

	sim.vision.ReadLinesFunc = func(
		context.Context, image.Rectangle, ports.TextLevel,
	) ([][]ports.TextPiece, error) {
		lines := make([][]ports.TextPiece, 0, len(words))
		for text, bounds := range words {
			lines = append(lines, []ports.TextPiece{{Text: text, Bounds: bounds}})
		}

		return lines, nil
	}
}

// The words and keys the vim journeys read and expect.
const (
	vimWordFile      = "File"
	vimWordEdit      = "Edit"
	vimWordExit      = "Exit"
	vimKeyReturn     = "Return"
	vimKeyExtend     = "Shift+Right"
	vimKeyExtendBack = "Shift+Left"
)

// TestSimulation_VimJourney covers vim mode. The screen's words are read as
// text, 0 and w move the cursor over them, a search highlights its matches,
// and Return clicks the character under the cursor and stays in vim mode.
func TestSimulation_VimJourney(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[vimHotkey] = []string{domain.ModeNameVim}

	sim := newSimHarness(t, cfg, nil)
	simWords(t, sim, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordEdit: image.Rect(200, 100, 260, 130),
		vimWordExit: image.Rect(100, 200, 160, 230),
	})

	cursorOn := func(want image.Rectangle) func() bool {
		return func() bool {
			frame, ok := sim.overlay.lastVimFrame()

			return ok && frame.Cursor == want
		}
	}

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("g", "g", "0")
	sim.waitFor("cursor on the F of File", cursorOn(image.Rect(100, 100, 115, 130)))

	sim.press("w")
	sim.waitFor("cursor on the E of Edit", cursorOn(image.Rect(200, 100, 215, 130)))

	sim.press("/", "x", "i")
	sim.waitFor("the match typed is highlighted", func() bool {
		frame, ok := sim.overlay.lastVimFrame()

		return ok && len(frame.Matches) == 1 && frame.Current == 0
	})

	sim.press("Return")
	sim.waitFor("cursor on the x of Exit", cursorOn(image.Rect(115, 200, 130, 230)))

	sim.press("Return")
	sim.waitFor("click recorded", func() bool { return len(sim.ax.recordedClicks()) > 0 })

	if got, want := sim.ax.recordedClicks()[0].point, image.Pt(122, 215); got != want {
		t.Fatalf("click landed at %v, want the x of Exit at %v", got, want)
	}

	if got := sim.app.CurrentMode(); got != domain.ModeVim {
		t.Fatalf("mode after the click = %v, want vim", got)
	}
}

// TestSimulation_VimClicks covers the clicks at the cursor. Return, gc, gr and
// the click ex commands click there, and a count clicks that many times, so
// 2gc is a double click, up to three. After each, vim mode stays open and
// reads the screen again. :leftdown and :leftup press and release, so a
// motion between them drags, and only the release reads the screen.
func TestSimulation_VimClicks(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordExit: image.Rect(100, 200, 160, 230),
	})

	var reads atomic.Int32

	read := sim.vision.ReadLinesFunc
	sim.vision.ReadLinesFunc = func(
		ctx context.Context, region image.Rectangle, level ports.TextLevel,
	) ([][]ports.TextPiece, error) {
		if level == ports.TextFast {
			reads.Add(1)
		}

		return read(ctx, region, level)
	}

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("g", "g", "0")
	vimFrameShows(sim, "cursor on the F of File", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(100, 100, 115, 130)
	})

	onFile, onExit := image.Pt(107, 115), image.Pt(107, 215)

	clicks := func(what string, keys []string, want ...simClick) {
		t.Helper()

		before, readsBefore := len(sim.ax.recordedClicks()), reads.Load()

		sim.press(keys...)
		sim.waitFor(what, func() bool {
			return len(sim.ax.recordedClicks()) == before+len(want) && reads.Load() > readsBefore
		})

		for index, got := range sim.ax.recordedClicks()[before:] {
			if got.action != want[index].action || got.point != want[index].point {
				t.Fatalf("%s: click %d = %v at %v, want %v at %v", what, index,
					got.action, got.point, want[index].action, want[index].point)
			}
		}

		if got := sim.app.CurrentMode(); got != domain.ModeVim {
			t.Fatalf("%s: mode after = %v, want vim", what, got)
		}
	}

	left := simClick{action: action.TypeLeftClick, point: onFile}
	right := simClick{action: action.TypeRightClick, point: onFile}

	clicks("Return clicks once", []string{vimKeyReturn}, left)
	clicks("2gc double clicks", []string{"2", "g", "c"}, left, left)
	clicks("3 Return triple clicks", []string{"3", vimKeyReturn}, left, left, left)
	clicks("5gc clicks three times", []string{"5", "g", "c"}, left, left, left)
	clicks("gr right clicks", []string{"g", "r"}, right)
	clicks(":leftclick 2 double clicks", vimExKeys("leftclick 2"), left, left)
	clicks(":rightclick right clicks once", vimExKeys("rightclick"), right)
	// A press reads nothing, since the screen changes on its release.
	readsBefore := reads.Load()

	sim.press(vimExKeys("leftdown")...)
	sim.waitFor(":leftdown presses", func() bool {
		recorded := sim.ax.recordedClicks()
		last := recorded[len(recorded)-1]

		return last.action == action.TypeLeftMouseDown && last.point == onFile
	})

	sim.press("j")
	vimFrameShows(sim, "cursor on the E of Exit", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(100, 200, 115, 230)
	})

	if got := reads.Load(); got != readsBefore {
		t.Fatalf("a press read the screen %d times, want none", got-readsBefore)
	}

	clicks(":leftup releases where the cursor moved", vimExKeys("leftup"),
		simClick{action: action.TypeLeftMouseUp, point: onExit})
}

// TestSimulation_VimCompletesExCommands covers Tab on the command line. It
// cycles through the commands that start with the name typed, listing them
// in the status bar, Shift+Tab goes back, and after the last it puts back the
// name typed.
func TestSimulation_VimCompletesExCommands(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	status := func(what, want string, keys ...string) {
		t.Helper()

		sim.press(keys...)
		vimFrameShows(sim, what, func(f ports.VimFrame) bool {
			return strings.Contains(f.Status, want)
		})
	}

	status(
		"the first match",
		":leftclick▏   [leftclick] leftdown leftup",
		"Shift+;",
		"l",
		"e",
		"f",
		"Tab",
	)
	status("the next match", ":leftdown▏   leftclick [leftdown] leftup", "Tab")
	status("back a match", ":leftclick▏   [leftclick] leftdown leftup", "Shift+Tab")
	status(
		"the name typed after the last",
		":lef▏   leftclick leftdown leftup",
		"Tab",
		"Tab",
		"Tab",
	)

	sim.press("Tab", vimKeyReturn)
	sim.waitFor(
		"the completed command clicked",
		func() bool { return len(sim.ax.recordedClicks()) == 1 },
	)
}

// TestSimulation_VimMovesTheHeldCursorWithGH covers gh in a session that
// holds the real cursor still. Motions leave it where it was, and gh moves it
// to the character under the vim cursor without a click, as :movemouse does.
func TestSimulation_VimMovesTheHeldCursorWithGH(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[vimHotkey] = []string{"vim --cursor-selection-mode hold"}

	sim := newSimHarness(t, cfg, nil)
	simWords(t, sim, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordEdit: image.Rect(200, 100, 260, 130),
	})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	held := sim.cursor.position()

	sim.press("0", "w")
	vimFrameShows(sim, "cursor on the E of Edit", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(200, 100, 215, 130)
	})

	if got := sim.cursor.position(); got != held {
		t.Fatalf("w moved the held cursor to %v, want it at %v", got, held)
	}

	sim.press("g", "h")
	sim.waitFor("the real cursor on the E of Edit", func() bool {
		return sim.cursor.position() == image.Pt(207, 115)
	})

	sim.press("0")
	vimFrameShows(sim, "cursor on the F of File", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(100, 100, 115, 130)
	})

	sim.press(vimExKeys("movemouse")...)
	sim.waitFor("the real cursor on the F of File", func() bool {
		return sim.cursor.position() == image.Pt(107, 115)
	})

	if clicks := sim.ax.recordedClicks(); len(clicks) != 0 {
		t.Fatalf("moving the cursor clicked %v", clicks)
	}

	if got := sim.app.CurrentMode(); got != domain.ModeVim {
		t.Fatalf("mode after the moves = %v, want vim", got)
	}
}

// vimExKeys are the keys that type line on vim's command line and run it.
func vimExKeys(line string) []string {
	keys := []string{"Shift+;"}

	for _, r := range line {
		if r == ' ' {
			keys = append(keys, "Space")
		} else {
			keys = append(keys, string(r))
		}
	}

	return append(keys, vimKeyReturn)
}

// TestSimulation_VimEscapeCancelsThenLeaves covers Escape in vim mode: it
// cancels a pending operator first, and leaves the mode only once nothing is
// pending.
func TestSimulation_VimEscapeCancelsThenLeaves(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[vimHotkey] = []string{domain.ModeNameVim}

	sim := newSimHarness(t, cfg, nil)
	simWords(t, sim, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("y")
	sim.waitFor("operator pending", func() bool {
		frame, ok := sim.overlay.lastVimFrame()

		return ok && frame.Pending
	})

	sim.press("Escape")
	sim.waitFor("operator canceled", func() bool {
		frame, ok := sim.overlay.lastVimFrame()

		return ok && !frame.Pending
	})

	if got := sim.app.CurrentMode(); got != domain.ModeVim {
		t.Fatalf("mode after the first Escape = %v, want vim", got)
	}

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	if clicks := sim.ax.recordedClicks(); len(clicks) != 0 {
		t.Fatalf("escape must not click, got %v", clicks)
	}
}

// vimHarness opens a harness with a vim binding and the screen reading words.
func vimHarness(t *testing.T, words map[string]image.Rectangle) *simHarness {
	t.Helper()

	cfg := simConfig()
	cfg.Hotkeys.Bindings[vimHotkey] = []string{domain.ModeNameVim}

	sim := newSimHarness(t, cfg, nil)
	simWords(t, sim, words)

	return sim
}

const vimHotkey = "Primary+Shift+V"

// vimFrameShows waits for a vim frame cond accepts.
func vimFrameShows(sim *simHarness, what string, cond func(ports.VimFrame) bool) {
	sim.waitFor(what, func() bool {
		frame, ok := sim.overlay.lastVimFrame()

		return ok && cond(frame)
	})
}

// TestSimulation_VimRefusesWhereTheScreenCannotBeRead covers a machine whose
// vision port reads no lines, such as Linux without tesseract's language data.
// Vim mode never opens, and the keyboard is free for the next mode.
func TestSimulation_VimRefusesWhereTheScreenCannotBeRead(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[vimHotkey] = []string{domain.ModeNameVim}

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(vimHotkey)
	sim.neverMode(domain.ModeVim, 250*time.Millisecond)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
}

// TestSimulation_VimRefusesBeforeAskingForAPermissionItCannotUse covers a
// platform that cannot read lines and whose screen capture permission is not
// granted yet, such as a portal desktop on Linux. The mode refuses without
// asking for the permission, which would have the user share the screen for
// a mode that can never open.
func TestSimulation_VimRefusesBeforeAskingForAPermissionItCannotUse(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[vimHotkey] = []string{domain.ModeNameVim}

	sim := newSimHarness(t, cfg, nil)

	var asked atomic.Int32

	sim.system.CheckScreenCapturePermissionFunc = func(context.Context) bool { return false }
	sim.system.RequestScreenCapturePermissionFunc = func(context.Context) ports.ScreenCaptureConsent {
		asked.Add(1)

		return ports.ScreenCaptureCanceled
	}

	sim.pressHotkey(vimHotkey)
	sim.neverMode(domain.ModeVim, 250*time.Millisecond)

	if asked.Load() != 0 {
		t.Fatalf("the screen capture permission was asked for %d times, want none", asked.Load())
	}
}

// TestSimulation_VimRefusesWhereTheOverlayCannotDrawIt covers a backend with
// no vim surface. The mode refuses rather than holding the keyboard over an
// empty overlay.
func TestSimulation_VimRefusesWhereTheOverlayCannotDrawIt(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[vimHotkey] = []string{domain.ModeNameVim}

	sim := newSimHarnessHeadlessOverlay(
		t,
		cfg,
		[]simDisplay{{name: mainDisplayName, bounds: simScreen}},
	)
	simWords(t, sim, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	sim.pressHotkey(vimHotkey)
	sim.neverMode(domain.ModeVim, 250*time.Millisecond)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
}

// TestSimulation_VimChangeTypesIntoTheApp covers editing through the app. ciw
// clicks the caret before the word and selects it a character at a time, the
// keys typed after go to the app and replace it, and Escape comes back to
// normal mode.
func TestSimulation_VimChangeTypesIntoTheApp(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordEdit: image.Rect(200, 100, 260, 130),
	})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("0", "w", "c", "i", "w")
	vimFrameShows(sim, "typing into the app", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Status, "INSERT")
	})

	sim.press("x")
	sim.waitFor("Edit selected and x typed over it", func() bool {
		return slices.Equal(sim.keyFeed.FedKeys(),
			[]string{vimKeyExtend, vimKeyExtend, vimKeyExtend, vimKeyExtend, "x"})
	})

	clicks := sim.ax.recordedClicks()
	if len(clicks) != 1 || clicks[0].action != action.TypeLeftClick ||
		clicks[0].point != image.Pt(200, 115) {
		t.Fatalf("the change did not click the caret before Edit: %v", clicks)
	}

	sim.press("Escape")
	vimFrameShows(sim, "back to normal mode", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Status, "NORMAL")
	})

	if got := sim.app.CurrentMode(); got != domain.ModeVim {
		t.Fatalf("mode after Escape from typing = %v, want vim", got)
	}
}

// TestSimulation_VimSubstitutes covers :s. While it is typed, each place it
// would change is highlighted and the status bar counts them. On Return it
// clicks the caret into the last change on the line only, and reaches the
// one before it with arrow keys, so the pointer does not jump to each.
func TestSimulation_VimSubstitutes(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordEdit: image.Rect(200, 100, 260, 130),
	})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	keys := vimExKeys("s/i/I/g")

	sim.press(keys[:len(keys)-1]...)
	vimFrameShows(sim, "both i highlighted and counted", func(f ports.VimFrame) bool {
		return strings.Contains(f.Status, "2 to change") &&
			slices.Equal(f.Matches, []image.Rectangle{
				image.Rect(115, 100, 130, 130), image.Rect(230, 100, 245, 130),
			})
	})

	// Back over the I typed, the le, the gap read as three spaces and the Ed.
	back := slices.Repeat([]string{"Left"}, 8)

	sim.press(vimKeyReturn)
	sim.waitFor("both i typed over", func() bool {
		return slices.Equal(sim.keyFeed.FedKeys(),
			slices.Concat([]string{vimKeyExtend, "I"}, back, []string{vimKeyExtendBack, "I"}))
	})

	clicks := sim.ax.recordedClicks()
	if len(clicks) != 1 || clicks[0].point != image.Pt(230, 115) {
		t.Fatalf("the substitute clicked %v, want once before the i of Edit", clicks)
	}
}

// vimSubstituteHarness opens vim mode over two lines, what and whom, whose
// screen shows whom changed to zom once landed is set.
func vimSubstituteHarness(t *testing.T, landed *atomic.Bool) *simHarness {
	t.Helper()

	first, second := image.Rect(100, 100, 160, 130), image.Rect(100, 200, 160, 230)

	sim := vimHarness(t, map[string]image.Rectangle{"what": first, "whom": second})
	sim.vision.ReadLinesFunc = func(
		context.Context, image.Rectangle, ports.TextLevel,
	) ([][]ports.TextPiece, error) {
		changed := "whom"
		if landed.Load() {
			changed = "zom"
		}

		return [][]ports.TextPiece{
			{{Text: "what", Bounds: first}},
			{{Text: changed, Bounds: second}},
		}, nil
	}

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	return sim
}

// TestSimulation_VimSubstituteWaitsForEachLineToShow covers :s across lines.
// It changes the last line first, and clicks the line above only once the
// screen shows the last one changed, since an app such as a browser can take
// a click ahead of the keys typed before it.
func TestSimulation_VimSubstituteWaitsForEachLineToShow(t *testing.T) {
	var landed atomic.Bool

	sim := vimSubstituteHarness(t, &landed)

	// The screen shows whom changed a while after its keys are typed, and
	// the key that runs :s is still being handled meanwhile.
	clicksBeforeLanding := make(chan int, 1)

	go func() {
		for len(sim.keyFeed.FedKeys()) < 3 {
			time.Sleep(5 * time.Millisecond)
		}

		time.Sleep(200 * time.Millisecond)

		clicksBeforeLanding <- len(sim.ax.recordedClicks())

		landed.Store(true)
	}()

	sim.press(vimExKeys("1,2s/wh/z/")...)
	sim.waitFor("what typed over once whom showed", func() bool {
		return len(sim.keyFeed.FedKeys()) == 6
	})

	if got := <-clicksBeforeLanding; got != 1 {
		t.Fatalf("%d clicks before the screen showed whom changed, want 1", got)
	}

	if clicks := sim.ax.recordedClicks(); len(clicks) != 2 || clicks[1].point.Y != 115 {
		t.Fatalf("clicks %v, want the second on the line of what", clicks)
	}
}

// TestSimulation_VimSubstituteStopsWhenTheScreenNeverShows covers :s across
// lines in an app that never shows a change. It stops after the first line
// and says how many places it left, rather than click ahead of its keys.
func TestSimulation_VimSubstituteStopsWhenTheScreenNeverShows(t *testing.T) {
	var landed atomic.Bool

	sim := vimSubstituteHarness(t, &landed)

	sim.press(vimExKeys("1,2s/wh/z/")...)
	vimFrameShows(sim, "the status says it stopped", func(f ports.VimFrame) bool {
		return strings.Contains(f.Status, "stopped with 1 left")
	})

	if clicks, keys := sim.ax.recordedClicks(), sim.keyFeed.FedKeys(); len(clicks) != 1 ||
		len(keys) != 3 {
		t.Fatalf("clicks %v and keys %v, want the second line's only", clicks, keys)
	}
}

// TestSimulation_VimOpensALineBelowAnIndentedOne covers o on a line read with
// padding before it. The new line gets no typed indent, since the padding is
// not text the app has, and Escape after typing leaves insert mode for good:
// the next key moves the cursor rather than typing into the app.
func TestSimulation_VimOpensALineBelowAnIndentedOne(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordExit: image.Rect(145, 200, 205, 230),
	})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("G", "o")
	vimFrameShows(sim, "typing into the app", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Status, "INSERT")
	})

	sim.press("x", "Escape")
	vimFrameShows(sim, "back in normal mode", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Status, "NORMAL") && !strings.Contains(f.Status, "one edit")
	})

	sim.press("w")
	vimFrameShows(sim, "still in normal mode after w", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Status, "NORMAL")
	})

	if got, want := sim.keyFeed.FedKeys(), []string{vimKeyReturn, "x"}; !slices.Equal(got, want) {
		t.Fatalf(
			"typed %v, want %v: a new line and what was typed on it, with no indent",
			got,
			want,
		)
	}
}

// TestSimulation_VimTypesOnTheLineOOpens covers o and O, which type into a
// line the screen does not show until vim reads it again. After o, the cursor
// lands on the last character typed on the new line below. O's line break
// leaves the app's caret on the line it pushed down, so it goes back onto the
// new line above before anything is typed.
func TestSimulation_VimTypesOnTheLineOOpens(t *testing.T) {
	file, exit, typed := image.Rect(
		100,
		100,
		160,
		130,
	), image.Rect(
		100,
		145,
		160,
		175,
	), image.Rect(
		100,
		190,
		130,
		220,
	)

	t.Run("o", func(t *testing.T) {
		sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: file, vimWordExit: exit})
		sim.vision.ReadLinesFunc = func(
			context.Context, image.Rectangle, ports.TextLevel,
		) ([][]ports.TextPiece, error) {
			lines := [][]ports.TextPiece{
				{{Text: vimWordFile, Bounds: file}},
				{{Text: vimWordExit, Bounds: exit}},
			}
			if slices.Contains(sim.keyFeed.FedKeys(), "y") {
				lines = append(lines, []ports.TextPiece{{Text: "xy", Bounds: typed}})
			}

			return lines, nil
		}

		sim.pressHotkey(vimHotkey)
		sim.waitMode(domain.ModeVim)

		sim.press("G", "o", "x", "y", "Escape")
		vimFrameShows(sim, "cursor on the y typed on the new line", func(f ports.VimFrame) bool {
			return strings.HasPrefix(f.Status, "NORMAL") &&
				f.Cursor == image.Rect(115, 190, 130, 220)
		})
	})

	t.Run("O", func(t *testing.T) {
		sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: file, vimWordExit: exit})

		sim.pressHotkey(vimHotkey)
		sim.waitMode(domain.ModeVim)

		sim.press("G", "O", "x", "y")
		sim.waitFor("typed onto the new line above", func() bool {
			return slices.Equal(sim.keyFeed.FedKeys(), []string{vimKeyReturn, "Left", "x", "y"})
		})
	})
}

// TestSimulation_VimLeavesTheLineBreakWhenDeletingPadding covers dw from
// column 0 of an indented line. Column 0 is the padding that stands for the
// indent on screen, so dw deletes only that, which the app does not have.
// Nothing is sent, since an erase with nothing selected takes the line break
// before the caret and joins the line to the one above.
func TestSimulation_VimLeavesTheLineBreakWhenDeletingPadding(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordExit: image.Rect(145, 200, 205, 230),
	})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("G", "0")
	vimFrameShows(sim, "cursor on the padding before Exit", func(f ports.VimFrame) bool {
		return f.Cursor.Min.Y == 200 && f.Cursor.Max.X <= 145
	})

	sim.press("d", "w")
	vimFrameShows(sim, "back in normal mode", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Status, "NORMAL")
	})

	if keys, clicks := sim.keyFeed.FedKeys(), sim.ax.recordedClicks(); len(keys) != 0 ||
		len(clicks) != 0 {
		t.Fatalf("deleting padding sent keys %v and clicks %v, want nothing", keys, clicks)
	}
}

// TestSimulation_VimHotkeysRunAheadOfVimButNotWhileTyping covers
// [vim.hotkeys]. A bound key runs its action and vim never sees it, so x here
// leaves the mode instead of deleting a character. While vim types into the
// app the same key types, as it would with nothing bound, and a chord bound
// globally too runs the global binding.
func TestSimulation_VimHotkeysRunAheadOfVimButNotWhileTyping(t *testing.T) {
	const shared = "Primary+Shift+K"

	cfg := simConfig()
	cfg.Hotkeys.Bindings[vimHotkey] = []string{domain.ModeNameVim}
	cfg.Hotkeys.Bindings[shared] = []string{"action left_click"}
	cfg.Vim.Hotkeys = map[string]config.StringOrStringArray{
		"x":    {config.CmdIdle},
		shared: {config.CmdIdle},
	}

	sim := newSimHarness(t, cfg, nil)
	simWords(t, sim, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordEdit: image.Rect(200, 100, 260, 130),
	})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("0", "w", "c", "i", "w")
	vimFrameShows(sim, "typing into the app", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Status, "INSERT")
	})

	sim.press("x")
	sim.waitFor("x typed into the app", func() bool {
		keys := sim.keyFeed.FedKeys()

		return len(keys) > 0 && keys[len(keys)-1] == "x"
	})

	if got := sim.app.CurrentMode(); got != domain.ModeVim {
		t.Fatalf("mode after typing a bound key = %v, want vim", got)
	}

	clicks := len(sim.ax.recordedClicks())

	sim.pressHotkey(shared)
	sim.waitFor("the global binding clicked", func() bool {
		return len(sim.ax.recordedClicks()) == clicks+1
	})

	if got := sim.app.CurrentMode(); got != domain.ModeVim {
		t.Fatalf("mode after the shared chord while typing = %v, want vim", got)
	}

	sim.press("Escape")
	vimFrameShows(sim, "back to normal mode", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Status, "NORMAL")
	})

	sim.press("x")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_VimEscapeLeavesTheCursorOnTheLastCharacterTyped covers the
// cursor after typing, which goes where vim leaves it: on the last character
// typed. The screen here does not change, so that is Edit's second character
// after two typed over its first.
func TestSimulation_VimEscapeLeavesTheCursorOnTheLastCharacterTyped(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordEdit: image.Rect(200, 100, 260, 130),
	})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("0", "w", "c", "i", "w")
	vimFrameShows(sim, "typing into the app", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Status, "INSERT")
	})

	sim.press("a", "b", "Escape")
	vimFrameShows(sim, "the cursor on Edit's second character", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Status, "NORMAL") && f.Cursor.Min.X == 215
	})
}

// TestSimulation_VimRefusesChangesWhereTypedKeysCannotReachTheApp covers a
// Linux Wayland session without the keyboard proxy, where Neru holds the
// keyboard through its own overlay and every key it typed would come back to
// it. A change, insert and undo are each refused before anything is clicked
// or typed, and the status line says why.
func TestSimulation_VimRefusesChangesWhereTypedKeysCannotReachTheApp(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordEdit: image.Rect(200, 100, 260, 130),
	})

	const reason = "typed keys cannot reach the app"

	sim.keyFeed.ReachesAppFunc = func() error {
		return derrors.New(derrors.CodeNotSupported, reason)
	}

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	for _, keys := range [][]string{{"0", "w", "c", "i", "w"}, {"i"}, {"u"}} {
		sim.press(keys...)
		vimFrameShows(
			sim,
			strings.Join(keys, "")+" refused in normal mode",
			func(f ports.VimFrame) bool {
				return strings.HasPrefix(f.Status, "NORMAL") && strings.Contains(f.Status, reason)
			},
		)
	}

	if fed := sim.keyFeed.FedKeys(); len(fed) != 0 {
		t.Fatalf("refused changes typed %v into the app", fed)
	}

	if clicks := sim.ax.recordedClicks(); len(clicks) != 0 {
		t.Fatalf("refused changes clicked the app: %v", clicks)
	}
}

// TestSimulation_VimDeleteLineStaysInTheField covers dd on an indented last
// line, such as a search bar under a title. The deletion starts at the line
// break above, so it clicks the end of the line and selects backwards, rather
// than clicking the line above, which is outside the field.
func TestSimulation_VimDeleteLineStaysInTheField(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordEdit: image.Rect(200, 140, 260, 170),
	})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("g", "g", "j", "d", "d")
	sim.waitFor("Edit and the line break before it selected backwards and erased", func() bool {
		return slices.Equal(sim.keyFeed.FedKeys(), []string{
			vimKeyExtendBack, vimKeyExtendBack, vimKeyExtendBack, vimKeyExtendBack,
			vimKeyExtendBack, "Delete",
		})
	})

	clicks := sim.ax.recordedClicks()
	if len(clicks) != 1 || clicks[0].point != image.Pt(262, 155) {
		t.Fatalf("dd did not click the end of Edit: %v", clicks)
	}
}

// TestSimulation_VimShiftVSelectsLines covers V as the Windows tap names it,
// "shift+v", with the modifier in lower case. It opens linewise visual mode,
// and j takes the next line in.
func TestSimulation_VimShiftVSelectsLines(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordEdit: image.Rect(100, 140, 160, 170),
	})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("g", "g", "shift+v", "j")
	vimFrameShows(sim, "both lines selected in linewise visual mode", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Status, "V-LINE") && len(f.Selection) == 2
	})
}

// TestSimulation_VimYankCopiesWithoutTouchingTheApp covers y. yiw copies the
// word as it was read, says so, and sends the app nothing.
func TestSimulation_VimYankCopiesWithoutTouchingTheApp(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordEdit: image.Rect(200, 100, 260, 130)})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("y", "i", "w")
	sim.waitFor("the word copied", func() bool {
		return slices.Equal(sim.clipboard.Written(), []string{vimWordEdit})
	})
	vimFrameShows(sim, "the copy said in the status bar", func(f ports.VimFrame) bool {
		return strings.Contains(f.Status, "copied 4 characters")
	})

	if keys, clicks := sim.keyFeed.FedKeys(), sim.ax.recordedClicks(); len(keys) != 0 ||
		len(clicks) != 0 {
		t.Fatalf("yank touched the app: keys %v, clicks %v", keys, clicks)
	}
}

// TestSimulation_VimDropsTheCopyFlashOfAClosedSession covers the timer that
// takes a yank's flash off the screen. A yank and then Escape before it fires
// close the session, and the timer must not draw it back.
func TestSimulation_VimDropsTheCopyFlashOfAClosedSession(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordEdit: image.Rect(200, 100, 260, 130)})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("y", "i", "w", "Escape")
	sim.waitMode(domain.ModeIdle)

	drawn := sim.overlay.vimDrawCount()

	time.Sleep(vimFlashForTest)

	if got := sim.overlay.vimDrawCount(); got != drawn {
		t.Fatalf("the flash timer drew %d vim frames after the session closed", got-drawn)
	}
}

// vimFlashForTest is longer than a yank's flash stays on screen.
const vimFlashForTest = 400 * time.Millisecond

// TestSimulation_VimDropsAnAccurateReadFromAClosedSession covers a slow
// accurate read that lands after its session closed and another opened. It
// must not replace the new session's text.
func TestSimulation_VimDropsAnAccurateReadFromAClosedSession(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	release := make(chan struct{})
	first := true

	var firstMu sync.Mutex

	sim.vision.ReadLinesFunc = func(
		ctx context.Context, region image.Rectangle, level ports.TextLevel,
	) ([][]ports.TextPiece, error) {
		firstMu.Lock()

		stale := level == ports.TextAccurate && first
		if stale {
			first = false
		}
		firstMu.Unlock()

		if stale {
			<-release

			return [][]ports.TextPiece{
				{{Text: "Stale", Bounds: image.Rect(500, 500, 575, 530)}},
			}, nil
		}

		return [][]ports.TextPiece{
			{{Text: vimWordFile, Bounds: image.Rect(100, 100, 160, 130)}},
		}, nil
	}

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)
	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)
	vimFrameShows(sim, "the new session read", func(f ports.VimFrame) bool {
		return !strings.Contains(f.Status, "reading")
	})

	close(release)
	sim.press("l")
	vimFrameShows(sim, "the cursor still on File", func(f ports.VimFrame) bool {
		return f.Cursor.In(image.Rect(100, 100, 160, 130))
	})
}

// TestSimulation_VimSaysWhenAnAccurateReadIsSlow covers an accurate read that
// takes long, which on macOS is a model compiling for a script met the first
// time. Past a second and a half a card says so, and it goes once the read
// lands.
func TestSimulation_VimSaysWhenAnAccurateReadIsSlow(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	release := make(chan struct{})

	sim.vision.ReadLinesFunc = func(
		ctx context.Context, _ image.Rectangle, level ports.TextLevel,
	) ([][]ports.TextPiece, error) {
		if level == ports.TextAccurate {
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		return [][]ports.TextPiece{
			{{Text: vimWordFile, Bounds: image.Rect(100, 100, 160, 130)}},
		}, nil
	}

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)
	vimFrameShows(sim, "the slow read said on screen", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Notice, "Preparing text recognition")
	})

	close(release)
	vimFrameShows(sim, "the notice gone once the read lands", func(f ports.VimFrame) bool {
		return f.Notice == "" && !strings.Contains(f.Status, "reading")
	})
}

// TestSimulation_VimJumpByLabel covers s. It takes any number of
// characters, labels each match over the character after it, and a label
// jumps there. Return jumps to the nearest match, and Backspace takes back a
// character. The machine makes the move, as it does for j, so j and k
// after a jump keep the jump's column.
func TestSimulation_VimJumpByLabel(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordEdit: image.Rect(200, 100, 260, 130),
		vimWordExit: image.Rect(100, 200, 160, 230),
	})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("s", "e", "x", "i", "Return")
	vimFrameShows(sim, "cursor on the E of Exit", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(100, 200, 115, 230) && len(f.Labels) == 0
	})

	// e matches three times, so the third match would get d after a and s.
	// d follows the e of Edit, so it is no label, and typing it adds to the
	// text.
	sim.press("s", "e")
	vimFrameShows(sim, "a label on each e, none of them d", func(f ports.VimFrame) bool {
		return len(f.Labels) == 3 && !slices.ContainsFunc(f.Labels, func(l ports.VimLabel) bool {
			return l.Text == "d"
		})
	})

	sim.press("d")
	vimFrameShows(sim, "d narrowed the jump to Edit", func(f ports.VimFrame) bool {
		return strings.Contains(f.Status, "sed") && len(f.Matches) == 1
	})
	sim.press("Escape")

	// k leaves the machine remembering the first column, which the jump
	// below has to replace.
	sim.press("k")
	vimFrameShows(sim, "cursor on the F of File", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(100, 100, 115, 130)
	})

	sim.press("s", "i", "t", "x", "Backspace")
	vimFrameShows(sim, "a label on each it", func(f ports.VimFrame) bool {
		return len(f.Labels) == 2 && len(f.Matches) == 2
	})

	frame, _ := sim.overlay.lastVimFrame()
	for _, label := range frame.Labels {
		if label.Bounds.Min.Y == 100 {
			sim.press(label.Text)
		}
	}

	vimFrameShows(sim, "cursor on the i of Edit", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(230, 100, 245, 130)
	})

	sim.press("j")
	vimFrameShows(sim, "cursor on the t of Exit, a shorter line", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(145, 200, 160, 230)
	})

	sim.press("k")
	vimFrameShows(sim, "cursor back on the i of Edit", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(230, 100, 245, 130)
	})
}

// TestSimulation_VimScrollsTheApp covers Ctrl+D. The whole screen is already
// in view, so it scrolls the app and reads the screen again.
func TestSimulation_VimScrollsTheApp(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	var reads atomic.Int32

	read := sim.vision.ReadLinesFunc
	sim.vision.ReadLinesFunc = func(
		ctx context.Context, region image.Rectangle, level ports.TextLevel,
	) ([][]ports.TextPiece, error) {
		if level == ports.TextFast {
			reads.Add(1)
		}

		return read(ctx, region, level)
	}

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("Ctrl+d")
	sim.waitFor("the app scrolled down", func() bool {
		scrolls := sim.ax.recordedScrolls()

		return len(scrolls) == 1 && scrolls[0].Y < 0
	})
	sim.waitFor("the screen read again", func() bool { return reads.Load() >= 2 })
}

// TestSimulation_VimTypesJAndKOnTheCommandLine covers j and k on the one
// line read, where they scroll the app as a motion. Typed on the command
// line they are text, so they scroll nothing.
func TestSimulation_VimTypesJAndKOnTheCommandLine(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("Shift+;", "j", "k")
	vimFrameShows(sim, "j and k on the command line", func(f ports.VimFrame) bool {
		return strings.Contains(f.Status, ":jk")
	})

	if scrolls := sim.ax.recordedScrolls(); len(scrolls) != 0 {
		t.Fatalf("typing on the command line scrolled the app %v", scrolls)
	}
}

// TestSimulation_VimScrollsTheAppSideways covers zl. It scrolls the app right
// by eight characters without a count and by the count with one, and touches
// nothing up or down.
func TestSimulation_VimScrollsTheAppSideways(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("z", "l")
	sim.waitFor(
		"the app scrolled sideways",
		func() bool { return len(sim.ax.recordedScrolls()) == 1 },
	)

	// A scroll key right after a scroll is a held key's repeat, and dropped.
	time.Sleep(vimScrollSettleForTest)
	sim.press("3", "z")
	vimFrameShows(sim, "the count and z waiting", func(f ports.VimFrame) bool {
		return strings.Contains(f.Status, "3z")
	})

	sim.press("l")
	sim.waitFor("the app scrolled sideways by the count", func() bool {
		return len(sim.ax.recordedScrolls()) == 2
	})

	scrolls := sim.ax.recordedScrolls()
	if scrolls[0].Y != 0 || scrolls[0].X == 0 || scrolls[1].Y != 0 ||
		scrolls[1].X*scrolls[1].X >= scrolls[0].X*scrolls[0].X {
		t.Fatalf("zl then 3zl scrolled %v, want sideways and 3zl shorter", scrolls)
	}
}

// TestSimulation_VimCentersTheCursorsLine covers zz. The word is near the
// top of the screen, so zz scrolls the app up by the distance from its line
// to the middle.
func TestSimulation_VimCentersTheCursorsLine(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("z", "z")
	sim.waitFor("the app scrolled", func() bool { return len(sim.ax.recordedScrolls()) == 1 })

	want := image.Pt(0, (simScreen.Dy()-30)/2-100)
	if got := sim.ax.recordedScrolls()[0]; got != want {
		t.Fatalf("zz scrolled %v, want %v", got, want)
	}
}

// TestSimulation_VimDotRepeatsTypedChange covers . after a change typed into
// the app. The machine never saw the typing, so . must replay it with the
// command, and change the word under the cursor the same way. A count before
// . replaces the command's own.
func TestSimulation_VimDotRepeatsTypedChange(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordEdit: image.Rect(200, 100, 260, 130),
	})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("0", "w", "c", "i", "w", "x", "Escape")
	vimFrameShows(sim, "back to normal mode", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Status, "NORMAL")
	})

	sim.press("0", ".")

	change := []string{vimKeyExtend, vimKeyExtend, vimKeyExtend, vimKeyExtend, "x"}
	sim.waitFor("the change repeated on File", func() bool {
		return slices.Equal(sim.keyFeed.FedKeys(), slices.Concat(change, change))
	})

	clicks := sim.ax.recordedClicks()
	if len(clicks) != 2 || clicks[1].point != image.Pt(103, 115) {
		t.Fatalf(". did not click the caret before File: %v", clicks)
	}

	vimFrameShows(sim, "normal mode after the repeat", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Status, "NORMAL")
	})

	// 2. is 2ciw, File and the spaces after it, with the x typed again.
	sim.press("0", "2", ".")
	sim.waitFor("the change repeated with the count", func() bool {
		fed := sim.keyFeed.FedKeys()
		if len(fed) <= 2*len(change) ||
			!slices.Equal(fed[:2*len(change)], slices.Concat(change, change)) {
			return false
		}

		third := fed[2*len(change):]
		selected := third[:len(third)-1]

		return len(selected) > len(change)-1 && third[len(third)-1] == "x" &&
			!slices.ContainsFunc(selected, func(k string) bool { return k != vimKeyExtend })
	})
}

// vimScrollSettleForTest is longer than vim mode waits after a scroll before
// it takes another scroll key.
const vimScrollSettleForTest = 200 * time.Millisecond

// TestSimulation_VimDropsScrollKeysThatRepeatFasterThanAScroll covers a held
// scroll key. Each scroll holds the keyboard while it settles and the screen
// is read again, so repeats queued behind one would keep the app scrolling
// long after the key is let go. The ones that arrive before a scroll has
// settled are dropped, and a later press scrolls again.
func TestSimulation_VimDropsScrollKeysThatRepeatFasterThanAScroll(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("Ctrl+d", "Ctrl+d", "Ctrl+d", "Ctrl+d", "Ctrl+d")
	sim.waitFor("one scroll", func() bool { return len(sim.ax.recordedScrolls()) >= 1 })

	if scrolls := sim.ax.recordedScrolls(); len(scrolls) != 1 {
		t.Fatalf("five repeats in a row scrolled %d times, want 1", len(scrolls))
	}

	time.Sleep(vimScrollSettleForTest)
	sim.press("Ctrl+d")
	sim.waitFor(
		"a later press scrolling again",
		func() bool { return len(sim.ax.recordedScrolls()) == 2 },
	)
}

// vimScrollGapForTest is a pace of scroll keys just slower than the fastest
// scroll vim mode takes.
const vimScrollGapForTest = 190 * time.Millisecond

// vimReadGate holds the fast reads after an activation until released or
// canceled, as a read of a screen still settling would run, and counts how
// each one ended.
type vimReadGate struct {
	armed     atomic.Bool
	release   chan struct{}
	started   atomic.Int32
	canceled  atomic.Int32
	completed atomic.Int32
	accurate  atomic.Int32
}

// gateVimReads wraps the fake's reads. Reads pass straight through until the
// gate is armed.
func gateVimReads(sim *simHarness) *vimReadGate {
	gate := &vimReadGate{release: make(chan struct{})}

	read := sim.vision.ReadLinesFunc
	sim.vision.ReadLinesFunc = func(
		ctx context.Context, region image.Rectangle, level ports.TextLevel,
	) ([][]ports.TextPiece, error) {
		if level == ports.TextAccurate {
			gate.accurate.Add(1)
		}

		if level == ports.TextFast && !region.Empty() && gate.armed.Load() {
			gate.started.Add(1)

			select {
			case <-gate.release:
			case <-ctx.Done():
				gate.canceled.Add(1)

				return nil, ctx.Err()
			}

			gate.completed.Add(1)
		}

		return read(ctx, region, level)
	}

	return gate
}

// TestSimulation_VimNeverRunsTwoReadsOfAScrolledScreenAtOnce covers a read
// that keeps capturing for a moment after it is canceled, as a KDE capture
// does while it holds the screen-cast session. The read the next scroll
// starts and the read a key makes for itself both wait for it to stop.
// Otherwise their capture would fail to take the session.
func TestSimulation_VimNeverRunsTwoReadsOfAScrolledScreenAtOnce(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordEdit: image.Rect(200, 100, 260, 130),
	})

	var (
		armed, inFlight, overlapped, finished atomic.Int32
		release                               = make(chan struct{})
	)

	read := sim.vision.ReadLinesFunc
	sim.vision.ReadLinesFunc = func(
		ctx context.Context, region image.Rectangle, level ports.TextLevel,
	) ([][]ports.TextPiece, error) {
		if level != ports.TextFast || region.Empty() || armed.Load() == 0 {
			return read(ctx, region, level)
		}

		if inFlight.Add(1) > 1 {
			overlapped.Add(1)
		}

		defer func() {
			inFlight.Add(-1)
			finished.Add(1)
		}()

		select {
		case <-release:
			return read(ctx, region, level)
		case <-ctx.Done():
			time.Sleep(vimLingeringCapture)

			return nil, ctx.Err()
		}
	}

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	armed.Store(1)
	sim.press("Ctrl+d")
	time.Sleep(vimScrollGapForTest)
	sim.press("Ctrl+d")
	time.Sleep(vimScrollGapForTest)

	// The key reads under the handler's lock, so the release comes from a
	// timer, once the read it cancels has had time to linger.
	time.AfterFunc(vimScrollGapForTest, func() { close(release) })
	sim.press("w")
	sim.waitFor("the reads of both scrolls and the key's own finished", func() bool {
		return finished.Load() == 3
	})

	if n := overlapped.Load(); n != 0 {
		t.Fatalf("%d reads of a scrolled screen started while another was still capturing", n)
	}
}

// vimLingeringCapture is how long a canceled read keeps capturing in
// TestSimulation_VimNeverRunsTwoReadsOfAScrolledScreenAtOnce.
const vimLingeringCapture = 60 * time.Millisecond

// TestSimulation_VimReadsOnlyTheScreenAScrollRunEndsOn covers scrolling a
// long page. Each Ctrl+D scrolls at once and keeps the cursor where it is on
// screen, so the overlay stays up as it was instead of blinking off and on
// with each scroll. The next scroll cancels the read of each screen scrolled
// past, so vim reads and refines only the screen the run ends on.
func TestSimulation_VimReadsOnlyTheScreenAScrollRunEndsOn(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})
	gate := gateVimReads(sim)

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)
	vimFrameShows(sim, "the activation's accurate read landed", func(f ports.VimFrame) bool {
		return !strings.Contains(f.Status, "reading")
	})

	before, _ := sim.overlay.lastVimFrame()

	gate.armed.Store(true)

	for range 3 {
		sim.press("Ctrl+d")
		time.Sleep(vimScrollGapForTest)
	}

	if scrolls := len(sim.ax.recordedScrolls()); scrolls != 3 {
		t.Fatalf("three spaced Ctrl+D scrolled %d times, want 3", scrolls)
	}

	vimFrameShows(
		sim,
		"the overlay left in place while the page moves",
		func(f ports.VimFrame) bool {
			return strings.Contains(f.Status, "reading") && !f.Cursor.Empty() &&
				f.Cursor == before.Cursor && f.Lens == before.Lens
		},
	)
	sim.waitFor("the reads of the two screens scrolled past canceled", func() bool {
		return gate.canceled.Load() == 2 && gate.started.Load() == 3
	})

	close(gate.release)
	vimFrameShows(sim, "the cursor back once the last screen is read", func(f ports.VimFrame) bool {
		return !f.Cursor.Empty() && !strings.Contains(f.Status, "reading")
	})

	if n := gate.completed.Load(); n != 1 {
		t.Fatalf("%d reads of scrolled screens landed, want only the last", n)
	}

	if n := gate.accurate.Load(); n != 2 {
		t.Fatalf("read accurately %d times, want once for the activation and once after", n)
	}
}

// TestSimulation_VimReadsOnlyAboveItsStatusBar covers the strip vim's status
// bar covers at the bottom of the window. The bar is drawn over the whole
// window, and vim reads only above it, so a capture never has to take the bar
// off the screen and the text hidden under it is not read.
func TestSimulation_VimReadsOnlyAboveItsStatusBar(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})
	sim.overlay.vimStatusHeight = 30

	var (
		readMu sync.Mutex
		read   []image.Rectangle
	)

	words := sim.vision.ReadLinesFunc
	sim.vision.ReadLinesFunc = func(
		ctx context.Context, region image.Rectangle, level ports.TextLevel,
	) ([][]ports.TextPiece, error) {
		if !region.Empty() {
			readMu.Lock()

			read = append(read, region)
			readMu.Unlock()
		}

		return words(ctx, region, level)
	}

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	frame, _ := sim.overlay.lastVimFrame()

	readMu.Lock()
	defer readMu.Unlock()

	if len(read) == 0 {
		t.Fatal("vim read nothing")
	}

	want := image.Rect(
		frame.Region.Min.X,
		frame.Region.Min.Y,
		frame.Region.Max.X,
		frame.Region.Max.Y-30,
	)
	for _, region := range read {
		if region != want {
			t.Fatalf(
				"vim read %v, want %v, the window %v above its status bar",
				region,
				want,
				frame.Region,
			)
		}
	}
}

// TestSimulation_VimMovesTheOverlayWithALineScroll covers Ctrl+E, which
// scrolls the text up a line under a cursor that stays on its word. Until the
// screen is read again, the overlay moves up with the text rather than
// blinking off.
func TestSimulation_VimMovesTheOverlayWithALineScroll(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})
	gate := gateVimReads(sim)

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	before, _ := sim.overlay.lastVimFrame()

	gate.armed.Store(true)
	sim.press("Ctrl+e")

	vimFrameShows(sim, "the cursor moved up with its text", func(f ports.VimFrame) bool {
		return strings.Contains(f.Status, "reading") &&
			f.Cursor.Size() == before.Cursor.Size() &&
			f.Cursor.Min.X == before.Cursor.Min.X && f.Cursor.Min.Y < before.Cursor.Min.Y
	})

	close(gate.release)
}

// TestSimulation_VimReadsFirstForAKeyAfterAScroll covers a motion right after
// a scroll, before the screen was read. The key acts on the screen as the
// scroll left it, so it cancels the read running in the background and reads
// that screen itself before it runs.
func TestSimulation_VimReadsFirstForAKeyAfterAScroll(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordEdit: image.Rect(200, 100, 260, 130),
	})
	gate := gateVimReads(sim)

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	gate.armed.Store(true)
	sim.press("Ctrl+d")
	sim.waitFor("the background read running", func() bool { return gate.started.Load() == 1 })

	gate.armed.Store(false)
	sim.press("0", "w")
	sim.waitFor("the background read the key replaced canceled", func() bool {
		return gate.canceled.Load() == 1
	})

	vimFrameShows(sim, "the cursor on Edit in the text read", func(f ports.VimFrame) bool {
		return f.Cursor.In(image.Rect(200, 100, 260, 130))
	})
}

// TestSimulation_VimEscapeCancelsTheReadOfAScrolledScreen covers leaving
// right after a scroll. The read of the scrolled screen is canceled, and no
// accurate read follows for a session that has closed.
func TestSimulation_VimEscapeCancelsTheReadOfAScrolledScreen(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})
	gate := gateVimReads(sim)

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)
	vimFrameShows(sim, "the activation's accurate read landed", func(f ports.VimFrame) bool {
		return !strings.Contains(f.Status, "reading")
	})

	gate.armed.Store(true)
	sim.press("Ctrl+d")
	sim.waitFor("the background read running", func() bool { return gate.started.Load() == 1 })

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
	sim.waitFor("the read canceled", func() bool { return gate.canceled.Load() == 1 })

	time.Sleep(vimScrollGapForTest)

	if n := gate.accurate.Load(); n != 1 {
		t.Fatalf("read accurately %d times, want only the activation's", n)
	}
}

// TestSimulation_VimCancelsAnAccurateReadOfAScreenScrolledAway covers an
// accurate read still running when the app scrolls. It is for a screen that
// is gone, so it is canceled, and a new one is made for the screen the
// scroll left.
func TestSimulation_VimCancelsAnAccurateReadOfAScreenScrolledAway(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	var accurate, canceled atomic.Int32

	read := sim.vision.ReadLinesFunc
	sim.vision.ReadLinesFunc = func(
		ctx context.Context, region image.Rectangle, level ports.TextLevel,
	) ([][]ports.TextPiece, error) {
		if level == ports.TextAccurate && accurate.Add(1) == 1 {
			<-ctx.Done()
			canceled.Add(1)

			return nil, ctx.Err()
		}

		return read(ctx, region, level)
	}

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)
	sim.waitFor(
		"the activation's accurate read running",
		func() bool { return accurate.Load() == 1 },
	)

	sim.press("Ctrl+d")
	sim.waitFor("that read canceled", func() bool { return canceled.Load() == 1 })
	sim.waitFor(
		"an accurate read of the scrolled screen",
		func() bool { return accurate.Load() == 2 },
	)
	vimFrameShows(sim, "it landed", func(f ports.VimFrame) bool {
		return !f.Cursor.Empty() && !strings.Contains(f.Status, "reading")
	})
}

// TestSimulation_VimLetsASlowAccurateReadFinish covers an accurate read past
// the point the overlay calls it slow, which on macOS is a model compiling
// for a new script. Canceling it would start the compile over, so a scroll
// leaves it running, and the read for the scrolled screen follows it.
func TestSimulation_VimLetsASlowAccurateReadFinish(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	var accurate, canceled atomic.Int32

	release := make(chan struct{})

	read := sim.vision.ReadLinesFunc
	sim.vision.ReadLinesFunc = func(
		ctx context.Context, region image.Rectangle, level ports.TextLevel,
	) ([][]ports.TextPiece, error) {
		if level == ports.TextAccurate && accurate.Add(1) == 1 {
			select {
			case <-release:
			case <-ctx.Done():
				canceled.Add(1)

				return nil, ctx.Err()
			}
		}

		return read(ctx, region, level)
	}

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)
	vimFrameShows(sim, "the slow read said on screen", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Notice, "Preparing text recognition")
	})

	sim.press("Ctrl+d")
	time.Sleep(vimScrollGapForTest)

	if n := canceled.Load(); n != 0 {
		t.Fatal("a scroll canceled an accurate read past the slow mark")
	}

	close(release)
	sim.waitFor("an accurate read of the scrolled screen after it", func() bool {
		return accurate.Load() == 2
	})
}

// TestSimulation_VimCountStopsAtFourDigits covers a long count, which would
// otherwise run a motion for minutes with the keyboard held. A fifth digit is
// dropped and the motion runs.
func TestSimulation_VimCountStopsAtFourDigits(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		vimWordFile: image.Rect(100, 100, 160, 130),
		vimWordEdit: image.Rect(200, 100, 260, 130),
	})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("0", "9", "9", "9", "9", "9")
	vimFrameShows(sim, "count held at four digits", func(f ports.VimFrame) bool {
		return strings.Contains(f.Status, "9999") && !strings.Contains(f.Status, "99999")
	})

	sim.press("w")
	vimFrameShows(sim, "the motion ran", func(f ports.VimFrame) bool {
		return strings.HasPrefix(f.Status, "NORMAL") && !strings.Contains(f.Status, "9999")
	})
}

// TestSimulation_VimMovesBetweenContainers covers containers. Two columns a
// gutter apart are two containers, the one the cursor is in is outlined, and
// Ctrl+W l moves to the other.
func TestSimulation_VimMovesBetweenContainers(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{
		"Menu":    image.Rect(100, 100, 160, 130),
		"Article": image.Rect(500, 100, 605, 130),
	})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("g", "g")
	vimFrameShows(sim, "the menu outlined", func(f ports.VimFrame) bool {
		return f.Container == image.Rect(100, 100, 160, 130)
	})

	sim.press("Ctrl+w", "l")
	vimFrameShows(sim, "the article outlined with the cursor in it", func(f ports.VimFrame) bool {
		return f.Container == image.Rect(500, 100, 605, 130) && f.Cursor.In(f.Container)
	})
}

// TestSimulation_VimKeepsTheFastReadWhereAnAccurateReadWouldMatch covers a
// platform whose accurate read reads what the fast one does, as Windows, and
// Linux with no second language, do. The screen is read once per read, and
// the icons the segment detector finds still join the text.
func TestSimulation_VimKeepsTheFastReadWhereAnAccurateReadWouldMatch(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	sim.vision.AccurateReadSame = true

	var accurateReads atomic.Int32

	read := sim.vision.ReadLinesFunc
	sim.vision.ReadLinesFunc = func(
		ctx context.Context, region image.Rectangle, level ports.TextLevel,
	) ([][]ports.TextPiece, error) {
		if level == ports.TextAccurate {
			accurateReads.Add(1)
		}

		return read(ctx, region, level)
	}

	sim.vision.DetectSegmentsFunc = func(
		context.Context, image.Rectangle, config.HintsSegmentConfig,
	) ([]*element.Element, error) {
		icon, err := element.NewElement(
			"segment-icon",
			image.Rect(180, 100, 210, 130),
			element.RoleButton,
		)

		return []*element.Element{icon}, err
	}

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)
	vimFrameShows(sim, "the refine landed", func(f ports.VimFrame) bool {
		return !strings.Contains(f.Status, "reading")
	})

	sim.press("0", "w")
	vimFrameShows(
		sim,
		"the cursor on the icon beside the fast read's text",
		func(f ports.VimFrame) bool {
			return f.Cursor == image.Rect(180, 100, 210, 130)
		},
	)

	if n := accurateReads.Load(); n != 0 {
		t.Fatalf("read the screen accurately %d times where it reads the same as fast", n)
	}
}

// TestSimulation_VimReachesIcons covers icons. An icon-only button the
// segment detector finds joins the text as a placeholder once the accurate
// read lands, w reaches it, Return clicks it, and a yank of its line leaves
// it out.
func TestSimulation_VimReachesIcons(t *testing.T) {
	sim := vimHarness(t, map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	sim.vision.DetectSegmentsFunc = func(
		context.Context, image.Rectangle, config.HintsSegmentConfig,
	) ([]*element.Element, error) {
		icon, err := element.NewElement(
			"segment-icon",
			image.Rect(180, 100, 210, 130),
			element.RoleButton,
		)

		return []*element.Element{icon}, err
	}

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)
	vimFrameShows(sim, "the accurate read landed", func(f ports.VimFrame) bool {
		return !strings.Contains(f.Status, "reading")
	})

	sim.press("y", "y")
	sim.waitFor("the line copied without the icon", func() bool {
		return slices.Equal(sim.clipboard.Written(), []string{vimWordFile})
	})

	sim.press("0", "w")
	vimFrameShows(sim, "the cursor on the icon", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(180, 100, 210, 130)
	})

	sim.press("Return")
	sim.waitFor("the icon clicked", func() bool {
		clicks := sim.ax.recordedClicks()

		return len(clicks) == 1 && clicks[0].point == image.Pt(195, 115)
	})
}
