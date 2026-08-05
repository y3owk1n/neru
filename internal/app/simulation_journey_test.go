package app_test

// Full-loop user-journey tests. Each test drives the real app with nothing
// but key strings — the same input the native event tap delivers — and
// asserts on the observable outcome: what was drawn, where the cursor went,
// and what was clicked or scrolled. See simulation_harness_test.go.

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
)

const (
	hintsHotkey  = "Primary+Shift+Space"
	gridHotkey   = "Primary+Shift+G"
	scrollHotkey = "Primary+Shift+S"
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
func TestSimulation_HintsJourney_ClickAction(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{"hints left_click"}

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
		t.Fatalf("click landed at %v, expected element center %v", clicks[0].point, save.Center())
	}

	if got := sim.cursor.position(); got != save.Center() {
		t.Fatalf("cursor at %v after click, expected element center %v", got, save.Center())
	}

	sim.waitMode(domain.ModeIdle)
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
func manyButtons(t *testing.T, count int) []*element.Element {
	t.Helper()

	elements := make([]*element.Element, 0, count)
	for index := range count {
		col := index % 4
		row := index / 4
		bounds := image.Rect(100+col*200, 100+row*100, 220+col*200, 140+row*100)
		elements = append(
			elements,
			simElement(t, fmt.Sprintf("btn-%d", index), bounds, fmt.Sprintf("Button %d", index)),
		)
	}

	return elements
}

// TestSimulation_HintsSearchJourney covers text search: "/" opens search,
// typing a query narrows the hints to matching elements, Return confirms,
// and selecting the surviving label lands on the matched element.
func TestSimulation_HintsSearchJourney(t *testing.T) {
	elements := threeButtons(t)
	sim := newSimHarness(t, simConfig(), elements)

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	// "/" dispatches search_hints asynchronously; wait for the search input to
	// appear before typing, or the query characters would be read as labels.
	sim.press("/")
	sim.waitFor("search input shown", func() bool {
		return sim.overlay.searchInputDrawCount() > 0
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
// default binding must be registered with the hotkey backend, canonicalized
// for the platform.
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
		{name: "MainDisplay", bounds: simScreen},
		{name: "SecondDisplay", bounds: second},
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
		return sim.overlay.monitorHideCount() > 0
	})
}

// TestSimulation_MonitorSelectNotSupported pins the platform-stub contract:
// on a backend without the MonitorSelector extension, activation reports
// CodeNotSupported, the mode never engages, and the app keeps working.
func TestSimulation_MonitorSelectNotSupported(t *testing.T) {
	sim := newSimHarnessHeadlessOverlay(t, monitorSelectConfig(), []simDisplay{
		{name: "MainDisplay", bounds: simScreen},
		{name: "SecondDisplay", bounds: image.Rect(1920, 0, 3840, 1080)},
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
	cfg := simConfig()
	cfg.HeldRepeat.Enabled = true
	cfg.HeldRepeat.InitialDelay = 5
	cfg.HeldRepeat.Interval = 5

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	// Key down without a release: the repeat engine takes over.
	sim.press("j")

	sim.waitFor("held key repeated at least 4 scrolls", func() bool {
		return len(sim.ax.recordedScrolls()) >= 4
	})

	// Release stops the repeat: wait for two identical samples 100ms apart.
	sim.press("__keyup_j")

	sim.waitFor("repeat stopped after key release", func() bool {
		before := len(sim.ax.recordedScrolls())

		time.Sleep(100 * time.Millisecond)

		return len(sim.ax.recordedScrolls()) == before
	})

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
