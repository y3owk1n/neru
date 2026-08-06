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
// One iteration is exactly one keystroke. Getting back to an empty input costs
// a keystroke of its own, and that one is deliberately outside the timer: it
// is dispatched on a goroutine (backspace is a mode hotkey), so timing it
// would measure a scheduler rather than the key path.

import (
	"image"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/domain"
)

// backspaceKey is the key bound to `action backspace` in the default mode
// hotkeys. It un-narrows a grid without leaving it.
const backspaceKey = "Backspace"

// benchSettleTimeout bounds the untimed wait for a backspace to land.
const benchSettleTimeout = 2 * time.Second

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

// firstGridLabelKey returns the lower-case first character of a drawn cell's
// label — the key a user would press to start narrowing to it.
func firstGridLabelKey(sim *simHarness) string {
	grid := sim.overlay.lastGrid()

	cells := grid.Cells()
	if len(cells) == 0 {
		sim.t.Fatal("grid drawn with zero cells")
	}

	label := cells[0].Coordinate()
	if label == "" {
		sim.t.Fatal("grid cell drawn without a label")
	}

	return strings.ToLower(string([]rune(label)[0]))
}
