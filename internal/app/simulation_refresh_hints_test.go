package app_test

import (
	"context"
	"image"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// refreshHintsKey is where these journeys bind refresh_hints inside hints mode.
const refreshHintsKey = "Ctrl+R"

// refreshHintsConfig binds hintsHotkey to activation and refreshHintsKey to
// refresh_hints.
func refreshHintsConfig(activation string) *config.Config {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{activation}
	cfg.Hints.Hotkeys[refreshHintsKey] = config.StringOrStringArray{"action refresh_hints"}

	return cfg
}

// runRefreshHints sends refresh_hints the way `neru action refresh_hints` does.
func runRefreshHints(sim *simHarness) ipc.Response {
	return sim.app.HandleCommand(context.Background(), ipc.Command{
		Action: domain.CommandAction,
		Args:   []string{"refresh_hints"},
	})
}

// TestSimulation_RefreshHintsPicksUpNewElementsAndKeepsTheFilter covers
// refresh_hints in a session opened with --role. The rescan labels an
// element that appeared since activation, and hands the scan the same filter
// the session was opened with rather than the default one.
func TestSimulation_RefreshHintsPicksUpNewElementsAndKeepsTheFilter(t *testing.T) {
	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, refreshHintsConfig("hints --role button"),
		[]*element.Element{save})

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("one label drawn", func() bool { return len(sim.overlay.lastHintLabels()) == 1 })

	sim.ax.setElements([]*element.Element{
		save,
		simElement(t, "cancel", image.Rect(300, 100, 420, 140), "Cancel"),
	})

	sim.press(refreshHintsKey)
	sim.waitFor("the new element labeled", func() bool {
		return len(sim.overlay.lastHintLabels()) == 2
	})

	if mode := sim.app.CurrentMode(); mode != domain.ModeHints {
		t.Fatalf("refresh left hints for %v", mode)
	}

	filters := sim.ax.scanFilters()
	filtered, refreshed := filters[0].Roles, filters[len(filters)-1].Roles

	if !slices.Equal(refreshed, filtered) {
		t.Fatalf("refresh scanned roles %v, want the session's %v", refreshed, filtered)
	}

	// Without the flag the scan gets the configured roles, so the comparison
	// above can tell a kept filter from a dropped one.
	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	resp := sim.app.HandleCommand(context.Background(), ipc.Command{Action: domain.ModeNameHints})
	if !resp.Success {
		t.Fatalf("plain hints activation failed: %s (%s)", resp.Message, resp.Code)
	}

	sim.waitMode(domain.ModeHints)

	filters = sim.ax.scanFilters()
	if unfiltered := filters[len(filters)-1].Roles; slices.Equal(unfiltered, filtered) {
		t.Fatalf(
			"the default scan used the filtered roles %v, so the test cannot tell them apart",
			filtered,
		)
	}
}

// TestSimulation_RefreshHintsStaysOnTheStrategyCycleEntryInUse covers
// refresh_hints in a session opened with a --strategy cycle. Pressing the
// activation binding again moves to the next entry, but a refresh rescans
// with the entry in use. Contour comes first, and only contour captures
// the screen, so a capture per scan shows the refresh stayed on it.
func TestSimulation_RefreshHintsStaysOnTheStrategyCycleEntryInUse(t *testing.T) {
	sim := newSimHarness(t, refreshHintsConfig("hints --strategy contour,axtree"), nil)
	sim.desktop.focusWindow(image.Rect(100, 100, 900, 700))

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("contour captured on entry", func() bool {
		return len(sim.vision.onScreenAtCaptures()) == 1
	})

	sim.press(refreshHintsKey)
	sim.waitFor("contour captured again on refresh", func() bool {
		return len(sim.vision.onScreenAtCaptures()) == 2
	})

	if mode := sim.app.CurrentMode(); mode != domain.ModeHints {
		t.Fatalf("refresh left hints for %v", mode)
	}
}

// TestSimulation_RefreshHintsReportsWhatItCouldNotDo covers the refusals.
// Outside hints mode there is no session to refresh, and a rescan that finds
// nothing leaves the mode, and both report a failure.
func TestSimulation_RefreshHintsReportsWhatItCouldNotDo(t *testing.T) {
	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, refreshHintsConfig("hints"), []*element.Element{save})

	if resp := runRefreshHints(sim); resp.Success {
		t.Fatalf("refresh_hints outside hints mode succeeded: %s", resp.Message)
	}

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	sim.ax.setElements(nil)

	if resp := runRefreshHints(sim); resp.Success {
		t.Fatalf("refresh_hints that found nothing succeeded: %s", resp.Message)
	}

	sim.waitMode(domain.ModeIdle)
}
