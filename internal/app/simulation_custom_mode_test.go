package app_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

// The declared mode these journeys enter, and the chord that enters it.
const (
	customModeHotkey = "Primary+Shift+W"
	customModeName   = "window"
)

// simConfigDeclaringAMode is the simulated configuration with one declared
// mode on top of the built-in ones: a label for the indicator, one bare key
// bound to a scroll so the journey can see it land, and a global chord that
// enters it by the mode word.
func simConfigDeclaringAMode() *config.Config {
	cfg := simConfig()

	hotkeys := config.DefaultCustomModeHotkeys()
	hotkeys["j"] = config.StringOrStringArray{"action scroll_down"}

	cfg.Modes = map[string]config.CustomModeConfig{
		customModeName: {Indicator: "Window", Hotkeys: hotkeys},
	}
	cfg.Hotkeys.Bindings[customModeHotkey] = []string{"mode " + customModeName}

	return cfg
}

// TestSimulation_CustomModeJourney is the whole path a declared mode exists
// for, as a user walks it: the global chord enters the mode, the indicator
// names it, a bare key runs the step the declaration bound, and Escape leaves
// with the indicator gone.
func TestSimulation_CustomModeJourney(t *testing.T) {
	sim := newSimHarness(t, simConfigDeclaringAMode(), nil)

	sim.pressHotkey(customModeHotkey)
	sim.waitMode(domain.ModeCustom)

	sim.waitFor("mode indicator shown", func() bool {
		visible, asked := sim.overlay.modeIndicatorVisibility()

		return asked && visible
	})

	sim.press("j")
	sim.waitFor("scroll recorded", func() bool { return len(sim.ax.recordedScrolls()) > 0 })

	if scrolls := sim.ax.recordedScrolls(); scrolls[0].Y == 0 {
		t.Fatalf("expected a vertical scroll for the declared 'j' binding, got %v", scrolls[0])
	}

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	sim.waitFor("mode indicator hidden", func() bool {
		visible, asked := sim.overlay.modeIndicatorVisibility()

		return asked && !visible
	})

	time.Sleep(4 * indicatorSettleWindow)

	if visible, _ := sim.overlay.modeIndicatorVisibility(); visible {
		t.Fatal("mode indicator was shown again after the declared mode exited")
	}
}

// TestSimulation_CustomModeSwitchesToABuiltInMode pins that a declared mode
// is a mode among the others: a step in its table enters scroll the way a
// hotkey would, and the keyboard is handed over rather than given back.
func TestSimulation_CustomModeSwitchesToABuiltInMode(t *testing.T) {
	cfg := simConfigDeclaringAMode()

	mode := cfg.Modes[customModeName]
	mode.Hotkeys["s"] = config.StringOrStringArray{"scroll"}
	cfg.Modes[customModeName] = mode

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(customModeHotkey)
	sim.waitMode(domain.ModeCustom)

	sim.press("s")
	sim.waitMode(domain.ModeScroll)

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}
