package app_test

// Full-loop user-journey tests. Each test drives the real app with nothing
// but key strings — the same input the native event tap delivers — and
// asserts on the observable outcome: what was drawn, where the cursor went,
// and what was clicked or scrolled. See simulation_harness_test.go.

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/config"
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

// The two fixture applications a journey switches between. The desktop starts
// focused on the first; the second is what it can switch away to.
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
// button without a mouse" journey: a binding with an explicit action
// ("hints left_click") makes typing the label move the cursor to the element
// center, click there, and drop back to idle.
//
// The three spellings of that binding run the same journey. A binding is text
// the daemon reads for itself, so this is where a flag that parses but never
// reaches the mode would show up — as a hint that highlights and then does
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

// TestSimulation_HintsEscape covers bailing out: Escape exits hints, hides
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

// TestSimulation_GridJourney covers the accessibility-independent path: grid
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
// user: typing in grid mode narrows what is on screen without redrawing the
// grid or putting the overlay up again. It asserts the domain value the user's
// keystroke produced — the prefix the cells are matched against — rather than
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

	// Labels are drawn upper case and the grid matches on them; a user types
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

// TestSimulation_GridLeavesNothingOnScreen pins the leaving half: exiting grid
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
// surface: every keystroke repaints it, and none of them puts the overlay up a
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

// TestSimulation_ScrollJourney covers scroll mode: j scrolls, Escape leaves
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
// after leaving a mode: the indicator that named it is gone. Scroll mode is
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
		visible, asked := sim.overlay.indicatorVisibility(ports.ModeIndicator)

		return asked && visible
	})

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	sim.waitFor("mode indicator hidden", func() bool {
		visible, asked := sim.overlay.indicatorVisibility(ports.ModeIndicator)

		return asked && !visible
	})

	// Nothing may put it back once the mode is gone: the polling goroutine is
	// stopped before the indicator is hidden, and a late tick would show it.
	time.Sleep(4 * indicatorSettleWindow)

	if visible, _ := sim.overlay.indicatorVisibility(ports.ModeIndicator); visible {
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
// hits on an empty screen: activation is abandoned, the app stays idle and
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

// TestSimulation_ExcludedApp covers the exclusion list: when the focused app
// is excluded, no mode activates.
func TestSimulation_ExcludedApp(t *testing.T) {
	sim := newSimHarness(t, simConfig(), threeButtons(t))
	sim.ax.setExcluded(true)

	sim.pressHotkey(hintsHotkey)
	sim.neverMode(domain.ModeHints, 250*time.Millisecond)

	if sim.overlay.hintDrawCount() != 0 {
		t.Fatal("hints must not be drawn for an excluded app")
	}
}

const recursiveGridHotkey = "Primary+Shift+C"

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

// TestSimulation_HintsSearchJourney covers text search: "/" opens search,
// typing a query narrows the hints to matching elements, Return confirms,
// and selecting the surviving label lands on the matched element.
//
// The second case runs the identical journey on an overlay that reports
// CodeNotSupported for the search input — the Linux overlay, where no backend
// draws that badge (#1328). The user sees no "/ sav 1 /" box and loses nothing
// else: the query still reaches the hints through the event tap's key stream.
// A refusal that surfaced as a failed mode, or that stopped the keystrokes
// getting through, is what this second case pins.
func TestSimulation_HintsSearchJourney(t *testing.T) {
	overlays := map[string]func(testing.TB, *config.Config, []*element.Element) *simHarness{
		"search input drawn":   newSimHarness,
		"search input refused": newSimHarnessRefusingHintSearch,
	}

	for name, newHarness := range overlays {
		t.Run(name, func(t *testing.T) {
			elements := threeButtons(t)
			sim := newHarness(t, simConfig(), elements)

			sim.pressHotkey(hintsHotkey)
			sim.waitMode(domain.ModeHints)
			sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

			// "/" dispatches search_hints asynchronously; wait for search to be
			// open before typing, or the query characters would be read as
			// labels. The wait is on the ask rather than on anything drawn,
			// because the refusing overlay draws nothing.
			sim.press("/")
			sim.waitFor("search input asked for", func() bool {
				return sim.overlay.searchInputAskCount() > 0
			})

			sim.typeLabel("sav") // matches only the "Save" button title

			sim.waitFor("hints narrowed to the query", func() bool {
				return len(sim.overlay.lastHintLabels()) == 1
			})

			sim.press("Return")

			labels := sim.overlay.lastHintLabels()
			if len(labels) != 1 {
				t.Fatalf("expected one hint after confirmed search, got %v", labels)
			}

			sim.typeLabel(labels[0])

			saveCenter := elements[0].Center()
			sim.waitFor("cursor on the searched element", func() bool {
				return sim.cursor.position() == saveCenter
			})
		})
	}
}

// TestSimulation_HintsTwoCharLabels covers label generation past the
// single-character alphabet: 12 elements with 9 hint characters force
// two-character labels; a first keystroke narrows the drawn set, and the
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

// TestSimulation_HintsRightClickAction covers a non-default pending action:
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

// TestSimulation_HintsRepeatJourney covers --repeat: after each click the
// mode re-arms with the action preserved, so consecutive selections click
// consecutively without re-pressing the hotkey.
func TestSimulation_HintsRepeatJourney(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{"hints left_click --repeat"}

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

// TestSimulation_HintsFlaggedBindingJourney covers a binding that carries a
// whole activation rather than a bare action: what to do, what to hold while
// doing it, and what to run once it is done.
//
// Each flag is asserted where a user would notice it — which button was
// clicked, what was held, and what ran afterwards. A flag that parses and then
// goes missing between the binding and the mode is invisible to the grammar's
// own tests, because the command still reads correctly and the mode still
// activates; this is the seam where it shows up.
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

// TestSimulation_RefusedBindingActivatesNothing covers the other half: a
// binding the grammar refuses enters no mode and draws nothing.
//
// Both refusals are ones a user used to experience as Neru being unreliable
// rather than as a mistake they had made. A typo activated the mode and did
// nothing else; "grid --search" activated grid and dropped the flag in silence,
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

// TestSimulation_RecursiveGridJourney covers the recursive grid: activation
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

// TestSimulation_ScrollDirections covers all four scroll directions with
// their default keys; each keypress must scroll on the right axis in the
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

// TestSimulation_HotkeyRegistration covers startup registration: every
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

// TestSimulation_DisabledModeBindingSkipped covers config gating: a binding
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

// TestSimulation_PassthroughExitsMode covers exit_after_passthrough: when a
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
// two-display desktop: activation draws one labeled panel per monitor, and
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

// TestSimulation_MonitorSelectNotSupported pins the platform-stub contract:
// on a backend without the MonitorSelector extension, activation reports
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

// TestSimulation_StickyModifierClick covers sticky modifiers: tapping Shift
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
	// label keystroke as a Shift chord; Neru strips it for label matching.
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

// TestSimulation_HeldRepeatScroll covers held-key repeat: holding j keeps
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
	// timer: the first scroll cannot arrive before the initial delay and each
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

	// Release stops the repeat: wait for two identical samples a
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

	// Move while holding: Down is bound to relative mouse movement.
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

// TestSimulation_ReleaseHeldButtonsOnExit pins the safety invariant: exiting
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

// TestSimulation_ThemeChangeReachesVisibleOverlay covers the journey the
// overlay's Style ownership exists for: hints are on screen when the system
// switches to dark mode, and the user sees them redrawn in the new theme
// rather than left in the old one until the mode is re-entered.
//
// The overlay owns config + theme -> Style, so at this seam the appearance is
// not observable and does not need to be: what reaches the overlay is one
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
// ownership: a reload notifies the overlay once, with the colors the reloaded
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

// TestSimulation_HintsNarrowingRedrawsWithoutShowingAgain pins the decision
// ADR 0003 rests on: entering hints puts the overlay on screen once, and every
// keystroke that narrows the labels redraws it without paying for the window
// sequence again. On macOS a show queues window-level, collection-behavior
// and ordering work on the main thread; doing that per keystroke is the
// latency regression AGENTS.md forbids.
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
	sim.waitFor("hints narrowed by prefix", func() bool {
		remaining := sim.overlay.lastHintLabels()

		return len(remaining) > 0 && len(remaining) < len(elements)
	})

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
// one mode replaces another: the mode they left is off the screen by the time
// the one they entered is on it, never both at once.
//
// It walks the four surfaces that draw — grid, hints, recursive grid and
// monitor-select — switching straight from each to the next without going
// through idle first, because a transition is where two overlays used to be
// left on screen together. The assertion is on what is drawn, not on the calls
// that got it there: which of them the overlay uses to realize a Frame is
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
// one transition that does not go through the common mode cleanup: scroll is
// entered without leaving the previous mode first, so that its event tap stays
// up. The monitor picker draws on panels of its own rather than on the shared
// surface, so nothing about clearing that surface takes them off the screen —
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

// TestSimulation_MonitorMoveRedrawsTheModeOnTheNewDisplay covers what a user
// sees when they send the cursor to another display with a mode open: the
// overlay leaves the display they came from and is back on the one they land
// on, with only that mode drawn.
//
// The path it exercises takes the frame off the screen before the warp and
// hands over a new one after it, across the handler lock and the monitor-move
// lock, which is why it is worth driving end to end rather than asserting on
// the calls in between.
func TestSimulation_MonitorMoveRedrawsTheModeOnTheNewDisplay(t *testing.T) {
	cfg := simConfig()
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
}

// TestSimulation_ScreenChangeWhileIdleLeavesTheOverlayHidden pins the quietest
// display change there is: plugging a monitor in with no mode open must not
// flash an overlay at the user.
//
// Idle is the state a user is in almost all of the time, so this is the screen
// change that happens most; the overlay window coming up for it would be
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
// heaviest thing a display change does: hints are open when the arrangement
// changes, and the labels a user is looking at have to come from the
// accessibility tree as it is now, drawn against the display as it is now.
//
// Stale labels here are worse than none: they point at positions that no
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

	// The display change relaid out the windows: one of the three buttons is
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

	// The user chose none of this: a display changing under them must leave
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
// half: the cells a user is about to type have to describe the display as it
// now is, and the selection they made on the display that is gone has to go
// with it — a cell coordinate from the old layout selects a different point on
// the new one.
//
// The selection is read off the pointer the grid draws for it, so the journey
// turns cursor-follow off first ("`"): with the cursor following, the selection
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
		pointer, drawn := sim.overlay.lastGridPointer(domain.ModeGrid)

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

	if pointer, _ := sim.overlay.lastGridPointer(domain.ModeGrid); pointer.Visible {
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
// the region away would throw their progress away with it — the mode is
// remapped onto the new display rather than restarted on it.
//
// "The same region" means the same fraction of the display, which is what the
// journey asserts: the zoomed rectangle covers the same proportion of the
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
// path: the display changed, hints were regenerated for it, and this time the
// accessibility tree has nothing to offer — the window the labels belonged to
// went with the display.
//
// Leaving the mode running would strand the user in hints with the old
// display's labels on screen, so the mode is left instead and the overlay comes
// down. Idle with nothing drawn is recoverable; a mode showing labels that
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

	// The app is still usable afterwards: a mode that needs no elements still
	// activates.
	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
}

// The display-change journeys above all have the mode open when the
// arrangement changes. The four below are the other half of that set: the
// change lands while the mode is *inactive*, and the mode has to come back
// built for the display that exists now rather than the one it last drew on.
//
// Nothing records that a display changed while a mode was away — each
// activation reads the current arrangement unconditionally — so what these
// pin is that eager rebuild. It is the whole mechanism protecting a user who
// docks, undocks or changes resolution between two activations, and without a
// journey on it, replacing it with anything cheaper would look free.
//
// Four rather than five: scroll draws no surface of its own, so it has no
// frame built for a display for a journey to read. Its overlay content is the
// indicators, which are placed by the adapter rather than described in a
// Frame.

// TestSimulation_GridComesBackOnTheDisplayThatChangedWhileItWasInactive covers
// the commonest shape of this: a user leaves grid mode, plugs a display in or
// changes resolution, and comes back to grid.
//
// A grid built for the display that is gone is worse than no grid: its cells
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

	// The mode is not on screen, so the change itself must draw nothing —
	// that half is TestSimulation_ScreenChangeWhileIdleLeavesTheOverlayHidden;
	// what matters here is that the rebuild happens on activation instead.
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
// they came from — a display change relays windows out, so the elements are
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

	// The display change relaid out the windows: one of the three buttons is
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
// a fresh start, so this asserts the opposite outcome — the whole of the new
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

	// "r" is the top-left cell of the default 3x3 key layout: the user got as
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
// arrangement: the picker's panels are the monitors, so a rearrangement while
// it is closed is a rearrangement of everything it draws.
//
// A panel describing a display that has been reconfigured is a target the user
// can pick and be moved to the wrong place by, which is the failure this rules
// out.
//
// Unlike its three siblings this asserts nothing about the shared overlay
// being visible, and deliberately: the picker draws on panels of its own and
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

	// Both displays, named individually: a count of two says nothing about
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

// TestSimulation_ThemeChangeRedrawsMonitorSelect closes the theme-change set:
// every other mode that draws is already pinned redrawing itself when the
// system switches between light and dark, and the monitor picker is the one
// that was not.
//
// It draws on panels of its own rather than the shared surface, so nothing
// about the other modes picking the new theme up brings it along: left out, it
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
// ADR 0005 is the decision that drove it to zero: the focused app is published
// by the application watcher and the keymap is settled when it changes, so a
// keystroke consults bindings that are already resolved.
//
// Asking which application is focused was the only thing on the keystroke path
// that left the process — on macOS it is a message to another application,
// which can be busy or wedged — and the handler holds the lock that serializes
// key handling, mode exit included, while it waits. The count is therefore
// what a user can be stalled by, which is why it is asserted here rather than
// described somewhere.
//
// Both directions are kept because they fail differently: the first is the
// regression this fixed, and the second is the mode that never paid, which a
// settle that asked unconditionally would newly charge.
//
// Grid mode drives both directions because it needs no accessibility tree of
// its own: nothing else in the keystroke being measured has any reason to ask
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

			// Entering the mode is allowed to ask — that is the one-shot the
			// keymap settles on. The grid is drawn before the mode starts
			// taking keys, so this waits for the event tap as well: the count
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
// without it: the daemon started the application watcher and made the hotkeys
// live before it registered the callback activations are delivered to, so an
// application switch in that window reached nobody, was never retried, and left
// the journey below entering the mode the *previous* application bound the key
// to. ADR 0005 carries why a dropped publication is unrecoverable rather than
// merely late.
//
// Worth stating what it is not, because the intermittency points elsewhere and
// #1348 named two suspects: neither goroutine `handleAppActivation` starts is
// involved. Publication is a lock-free cell write on the caller's goroutine, so
// it is already in place before the keystroke that reads it; the passthrough
// refresh only reads; and the hotkey refresh is deferred outright while a mode
// is open. What was intermittent was whether the daemon had finished starting,
// not what it did once it had.
//
// It asserts the order rather than the symptom because the order is what makes
// the symptom impossible: whether the window is wide enough to fall into is a
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

// TestSimulation_FocusChangeMidModeRebindsTheKey covers switching applications
// without leaving the mode — passing a shortcut through to the system and
// landing somewhere else is the ordinary way it happens — and pins what the
// next keystroke does: it runs the binding the newly focused application's
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

	sim.waitFor("a binding for the shared key to run", func() bool {
		return sim.app.CurrentMode() != domain.ModeGrid
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
// other half of the journey above, and the one #1252 reported: knowing what
// the next key means is worth nothing if the key never arrives.
//
// With passthrough on, the event tap passes every modifier chord to the
// focused application except the ones it is told to keep. Those are the chords
// the mode binds — which per-app overrides make a function of the focused
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
// drawn with" while it is empty, and a load settles it — after which a settled
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

	// The labels the user reads, not just the strings they were built from: a
	// coordinate outside the new characters is a cell nobody can type.
	for _, cell := range cells {
		if strings.ContainsFunc(cell.Coordinate(), func(r rune) bool {
			return !strings.ContainsRune("ASDF", r)
		}) {
			t.Fatalf("cell labeled %q, which the new characters cannot spell", cell.Coordinate())
		}
	}
}
