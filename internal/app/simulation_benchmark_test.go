package app_test

// Per-keystroke cost of the modes whose overlay narrows while the user types.
//
// The event tap sits on every keystroke, so key handling getting slower is a
// regression (AGENTS.md), and ADR 0003 keeps the grid's incremental updates on
// a direct call path for exactly that reason. These benchmarks are what makes
// "unchanged" a measurement rather than a claim: they drive the real
// application through the simulation harness, so a keystroke costs everything
// it costs in production except the native draw.
//
// One iteration is exactly one keystroke. Where getting back to where the
// iteration started costs a keystroke of its own, that one is deliberately
// outside the timer: it is dispatched on a goroutine (backspace is a mode
// hotkey), so timing it would measure a scheduler rather than the key path.
//
// The hints benchmark measures the other thing a keystroke can cost: deciding
// what a key is bound to while the mode declares per-app hotkey overrides. That
// used to ask the operating system which application is focused — on macOS a
// message to another process, made under the lock that serializes key handling
// — and since ADR 0005 it consults a keymap settled when the focused app
// changed. None of these gate continuous integration — a timing threshold on a
// three-operating-system matrix is a source of flakes, and the durable
// guarantee is the call count the journeys assert, not the nanoseconds.

import (
	"image"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

// backspaceKey is the key bound to `action backspace` in the default mode
// hotkeys. It un-narrows a grid without leaving it.
const backspaceKey = "Backspace"

// benchSettleTimeout bounds the untimed wait for a backspace to land.
const benchSettleTimeout = 2 * time.Second

// benchHintElements is the size of the hint set the hints keystroke is
// measured against: more elements than there are hint characters, so the labels
// are the mixed one- and two-character set a real screen produces.
const benchHintElements = 12

// unmatchedHintKey is a letter outside the default hint alphabet, so it is a
// prefix of no label on screen.
const unmatchedHintKey = "z"

// BenchmarkGridNarrowingKeystroke measures one narrowing keystroke in grid
// mode: the event tap hands a key to the handler, the grid manager matches it,
// and the overlay is told the new prefix.
func BenchmarkGridNarrowingKeystroke(b *testing.B) {
	sim := newSimHarness(b, simConfig(), nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	key := firstGridLabelKey(sim)
	drawsBefore := sim.overlay.gridDrawCount()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		sim.press(key)

		b.StopTimer()
		clearGridNarrowing(sim)
		b.StartTimer()
	}

	b.StopTimer()

	if mode := sim.app.CurrentMode(); mode != domain.ModeGrid {
		b.Fatalf("benchmark left grid mode for %s; the numbers are not grid's",
			domain.ModeString(mode))
	}

	// Narrowing must never redraw the grid — that is the property being
	// measured, and a benchmark that redrew would be measuring the wrong path.
	if drawn := sim.overlay.gridDrawCount() - drawsBefore; drawn != 0 {
		b.Fatalf("grid redrawn %d times while narrowing; the keystrokes took the draw path",
			drawn)
	}
}

// BenchmarkGridSelectionKeystroke measures the other keystroke grid mode has:
// the one that completes a cell's label. It opens the subgrid inside that cell
// *and* moves the selection onto it, and on a backend that paints both into one
// surface — every Linux one — that used to be two full repaints of it (#1492).
//
// The session has cursor-follow turned off, because that is the one this is
// about: with the real cursor riding the selection there is no pointer stand-in
// to move and nothing was ever paid twice.
//
// surface-updates/op is the number that matters and the one a host without a
// display can still produce: how many times the keystroke asked the grid
// surface to change. Time and allocations are reported beside it so a change
// that bought a repaint with a slower key path cannot hide.
func BenchmarkGridSelectionKeystroke(b *testing.B) {
	sim := newSimHarness(b, simConfig(), nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	stopCursorFollowing(sim)

	cells := sim.overlay.lastGrid().Cells()
	if len(cells) == 0 {
		b.Fatal("grid drawn with zero cells")
	}

	label := []rune(cells[0].Coordinate())
	if len(label) < 2 {
		b.Fatalf("grid label %q is one character; it has no keystroke before the selection",
			string(label))
	}

	// The narrowing half is typed once, outside the loop: each iteration
	// re-enters the subgrid from the same prefix, which is where backing out of
	// one leaves the input.
	sim.typeLabel(string(label[:len(label)-1]))

	selectionKey := strings.ToLower(string(label[len(label)-1]))
	subgridsBefore := sim.overlay.subgridCount()

	// Counted per iteration rather than over the whole run, because backing out
	// of the subgrid re-runs the update callback and its surface updates are
	// not the keystroke's. Both reads are outside the timer.
	surfaceUpdates := 0

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		b.StopTimer()

		updatesBefore := sim.overlay.gridSurfaceUpdates()

		b.StartTimer()

		sim.press(selectionKey)

		b.StopTimer()

		surfaceUpdates += sim.overlay.gridSurfaceUpdates() - updatesBefore

		backOutOfSubgrid(sim)
		b.StartTimer()
	}

	b.StopTimer()

	if mode := sim.app.CurrentMode(); mode != domain.ModeGrid {
		b.Fatalf("benchmark left grid mode for %s; the numbers are not grid's",
			domain.ModeString(mode))
	}

	// Every timed keystroke has to have opened a subgrid, or the benchmark
	// measured keys that selected nothing.
	if opened := sim.overlay.subgridCount() - subgridsBefore; opened != b.N {
		b.Fatalf("%d subgrids opened for %d keystrokes; some keys selected nothing",
			opened, b.N)
	}

	b.ReportMetric(float64(surfaceUpdates)/float64(b.N), "surface-updates/op")
}

// BenchmarkRecursiveGridKeystroke measures one keystroke in recursive-grid
// mode, which unlike grid repaints its whole surface on every key.
func BenchmarkRecursiveGridKeystroke(b *testing.B) {
	sim := newSimHarness(b, simConfig(), nil)

	sim.pressHotkey(recursiveGridHotkey)
	sim.waitMode(domain.ModeRecursiveGrid)
	sim.waitFor("recursive grid drawn", func() bool {
		_, ok := sim.overlay.lastRecursiveGridBounds()

		return ok
	})

	full, _ := sim.overlay.lastRecursiveGridBounds()
	drawsBefore := sim.overlay.recursiveGridDrawCount()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		sim.press("r")

		b.StopTimer()
		climbBackOut(sim, full)
		b.StartTimer()
	}

	b.StopTimer()

	if mode := sim.app.CurrentMode(); mode != domain.ModeRecursiveGrid {
		b.Fatalf("benchmark left recursive-grid mode for %s; the numbers are not its",
			domain.ModeString(mode))
	}

	// Every timed keystroke has to have zoomed, or the benchmark spent its
	// iterations at a depth that could no longer divide.
	if drawn := sim.overlay.recursiveGridDrawCount() - drawsBefore; drawn < b.N {
		b.Fatalf("recursive grid redrawn %d times for %d keystrokes; some did not zoom",
			drawn, b.N)
	}
}

// BenchmarkHintsKeystroke measures one keystroke in hints mode — the mode a
// user spends most of their keystrokes in — configured the way that used to
// make a keystroke expensive: the mode declares per-app hotkey overrides, which
// before ADR 0005 meant asking the operating system which application is
// focused before the handler could decide what the key was bound to, holding
// the lock that serializes key handling while it waited. Leaving the overrides
// out would measure a mode that never took that path, and would keep reporting
// the same number if the settled keymap were quietly given up.
//
// The key it presses is a prefix of no label, and that is the point rather than
// a shortcut: it pays for the whole key path — the keymap lookup, the hint
// filter and the redraw — and leaves the drawn set
// exactly as it found it, so every iteration measures the same keystroke and
// none of them needs untimed work in between. A narrowing keystroke could not:
// changing how many hints are drawn is a structural change, which the hint
// manager deliberately debounces onto a timer, so its redraw would land outside
// the iteration that caused it and the reset between iterations would cost far
// more than the thing being measured.
func BenchmarkHintsKeystroke(b *testing.B) {
	cfg := simConfig()
	cfg.Hints.AppConfigs = []config.AppConfig{
		perAppHotkeyOverride(simFixtureBundleID, unpressedOverrideKey, "action left_click"),
	}

	sim := newSimHarness(b, cfg, manyButtons(b, benchHintElements))

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	if labeled := len(sim.overlay.lastHintLabels()); labeled != benchHintElements {
		b.Fatalf("hints drawn for %d of %d elements; the fixture is not the one being measured",
			labeled, benchHintElements)
	}

	// The activation is what settles the keymap, and it may ask the platform
	// which app is focused while it does. Waiting for the mode to be taking
	// keys keeps that out of the measured window.
	sim.waitFor("the mode taking keystrokes", sim.tap.IsEnabled)

	drawsBefore := sim.overlay.hintDrawCount()
	movesBefore := sim.cursor.moveCount()
	queriesBefore := sim.ax.focusedAppQueryCount()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		sim.press(unmatchedHintKey)
	}

	b.StopTimer()

	if mode := sim.app.CurrentMode(); mode != domain.ModeHints {
		b.Fatalf("benchmark left hints mode for %s; the numbers are not hints'",
			domain.ModeString(mode))
	}

	// A key that matched a label would have selected an element and moved the
	// cursor — a different, and much more expensive, path than the one claimed.
	if moved := sim.cursor.moveCount() - movesBefore; moved != 0 {
		b.Fatalf("cursor moved %d times; the keystrokes selected hints instead of missing them",
			moved)
	}

	// Each keystroke repaints the full set exactly once. Anything else means
	// the iterations were not the keystroke this claims to measure.
	if drawn := sim.overlay.hintDrawCount() - drawsBefore; drawn != b.N {
		b.Fatalf("hints redrawn %d times for %d keystrokes; the keystrokes did not all reach "+
			"the surface", drawn, b.N)
	}

	// What the measured keystroke no longer costs: a question to the operating
	// system. It was one per keystroke before ADR 0005, and the nanoseconds
	// above are only comparable across that change because this is zero.
	if asked := sim.ax.focusedAppQueryCount() - queriesBefore; asked != 0 {
		b.Fatalf("%d keystrokes asked which app is focused %d times, want 0; the keymap was "+
			"resolved on the keystroke rather than settled", b.N, asked)
	}
}

// clearGridNarrowing takes the grid back to an empty input without leaving the
// mode. Backspace is a mode hotkey, so the app runs it on a goroutine and this
// waits for it to land.
func clearGridNarrowing(sim *simHarness) {
	sim.press(backspaceKey)

	settle(sim, "grid input cleared", func() bool {
		prefix, narrowed := sim.overlay.lastMatchPrefix()

		return narrowed && prefix == ""
	})
}

// backOutOfSubgrid leaves an open subgrid without leaving grid mode, which puts
// the input back at the prefix the selection keystroke was pressed from.
// Backspace is a mode hotkey, so the app runs it on a goroutine and this waits
// for the grid it restores to be drawn.
func backOutOfSubgrid(sim *simHarness) {
	drawn := sim.overlay.gridDrawCount()

	sim.press(backspaceKey)

	settle(sim, "the subgrid closed", func() bool {
		return sim.overlay.gridDrawCount() > drawn
	})
}

// climbBackOut backtracks the recursive grid to the region it started from.
func climbBackOut(sim *simHarness, full image.Rectangle) {
	sim.press(backspaceKey)

	settle(sim, "recursive grid backtracked", func() bool {
		bounds, drawn := sim.overlay.lastRecursiveGridBounds()

		return drawn && bounds == full
	})
}

// settle spins until cond holds. It is the untimed counterpart of waitFor,
// polling far more tightly because a benchmark runs it once per iteration.
func settle(sim *simHarness, desc string, cond func() bool) {
	deadline := time.Now().Add(benchSettleTimeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		runtime.Gosched()
	}

	sim.t.Fatalf("timed out waiting for %s", desc)
}
