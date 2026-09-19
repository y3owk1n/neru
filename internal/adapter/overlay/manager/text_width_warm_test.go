package manager_test

import (
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

// warmTestHintsFamily is the face the hint draws below are made in.
const warmTestHintsFamily = "Warm Test Hints"

// countingMeasurer counts how often the platform's text layer is asked.
type countingMeasurer struct{ calls atomic.Int64 }

func (m *countingMeasurer) Measure(
	text, _ string,
	size float64,
	_ bool,
) (ports.TextMetrics, error) {
	m.calls.Add(1)

	return ports.TextMetrics{Width: float64(utf8.RuneCountInString(text)) * size, Height: size}, nil
}

// TestWarmBadgeTextWidths_LeavesADrawNothingToMeasure pins what the warming is
// for: hint narrowing and the search badge redraw on every keystroke, so the
// characters they size their boxes around are measured when the configuration
// arrives and never with a key waiting on it.
func TestWarmBadgeTextWidths_LeavesADrawNothingToMeasure(t *testing.T) {
	measurer := &countingMeasurer{}

	ports.SetTextMeasurer(measurer)
	t.Cleanup(func() { ports.SetTextMeasurer(nil) })

	cfg := config.DefaultConfig()
	cfg.Hints.UI.FontFamily = warmTestHintsFamily
	cfg.Hints.SearchInputUI.FontFamily = "Warm Test Search"
	cfg.ModeIndicator.UI.FontFamily = "Warm Test Indicator"
	cfg.RecursiveGrid.UI.FontFamily = "Warm Test Plates"

	manager.WarmBadgeTextWidths(cfg)

	warmed := measurer.calls.Load()
	if warmed == 0 {
		t.Fatal("WarmBadgeTextWidths measured nothing")
	}

	draws := []struct {
		text, family string
		bold         bool
	}{
		{"AS", warmTestHintsFamily, true},
		{"JKL", warmTestHintsFamily, true},
		{"/ save file  12", "Warm Test Search", false},
		{"Scroll", "Warm Test Indicator", true},
		{"R", "Warm Test Plates", false},
	}

	for _, draw := range draws {
		// Any size and any display scale: a face is warmed once for all of them.
		for _, size := range []float64{10, 12.5, 20, 30} {
			badge.TextWidth(draw.text, ports.ResolveFont(draw.family), size, draw.bold)
		}
	}

	if asked := measurer.calls.Load(); asked != warmed {
		t.Fatalf("draws measured %d more times after warming, want none", asked-warmed)
	}
}

func TestWarmBadgeTextWidths_NoConfigurationIsNothingToWarm(t *testing.T) {
	measurer := &countingMeasurer{}

	ports.SetTextMeasurer(measurer)
	t.Cleanup(func() { ports.SetTextMeasurer(nil) })

	manager.WarmBadgeTextWidths(nil)

	if asked := measurer.calls.Load(); asked != 0 {
		t.Fatalf("measured %d times with no configuration, want 0", asked)
	}
}
