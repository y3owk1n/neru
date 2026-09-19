package badge_test

import (
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/ports"
)

// countingMeasurer makes a W twice as wide as anything else, and counts how
// often it is asked.
type countingMeasurer struct {
	mu    sync.Mutex
	calls int
}

func (m *countingMeasurer) Measure(
	text, _ string,
	size float64,
	_ bool,
) (ports.TextMetrics, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()

	var width float64

	for _, character := range text {
		if character == 'W' {
			width += size
		} else {
			width += size / 2
		}
	}

	return ports.TextMetrics{Width: width, Height: size}, nil
}

func (m *countingMeasurer) asked() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.calls
}

func TestTextWidth(t *testing.T) {
	t.Run("a wide label gets a wider box than a narrow one", func(t *testing.T) {
		installMeasurer(t, &countingMeasurer{})

		wide := badge.TextWidth("WW", "Width Test A", 10, false)
		narrow := badge.TextWidth("II", "Width Test A", 10, false)

		if wide != 20 || narrow != 10 {
			t.Fatalf("TextWidth() = %d for WW and %d for II, want 20 and 10", wide, narrow)
		}

		// The estimate this replaces could not tell them apart.
		if badge.EstimateTextWidth("WW", 10) != badge.EstimateTextWidth("II", 10) {
			t.Fatal("the estimate tells WW from II, so this test pins nothing")
		}
	})

	t.Run("the platform is asked once per character, not once per label", func(t *testing.T) {
		measurer := &countingMeasurer{}
		installMeasurer(t, measurer)

		for range 50 {
			for _, label := range []string{"AS", "SA", "AA", "SS", "ASA"} {
				badge.TextWidth(label, "Width Test B", 12, true)
			}
		}

		if asked := measurer.asked(); asked != 2 {
			t.Fatalf("measured %d times for an alphabet of 2, want 2", asked)
		}
	})

	t.Run("a font is a family, a size and a weight", func(t *testing.T) {
		measurer := &countingMeasurer{}
		installMeasurer(t, measurer)

		badge.TextWidth("A", "Width Test C", 10, false)
		badge.TextWidth("A", "Width Test C", 11, false)
		badge.TextWidth("A", "Width Test C", 10, true)
		badge.TextWidth("A", "Width Test D", 10, false)

		if asked := measurer.asked(); asked != 4 {
			t.Fatalf("measured %d times for 4 fonts, want 4", asked)
		}
	})

	t.Run("a rune outside ASCII is measured too", func(t *testing.T) {
		installMeasurer(t, &countingMeasurer{})

		if got := badge.TextWidth("●", "Width Test E", 10, false); got != 5 {
			t.Fatalf("TextWidth() = %d, want 5", got)
		}
	})

	t.Run("a platform that cannot measure answers with the estimate", func(t *testing.T) {
		installMeasurer(t, nil)

		got := badge.TextWidth("WW", "Width Test F", 10, false)
		if want := badge.EstimateTextWidth("WW", 10); got != want {
			t.Fatalf("TextWidth() = %d, want the estimate %d", got, want)
		}
	})

	t.Run("nothing to draw is as wide as the estimate says", func(t *testing.T) {
		installMeasurer(t, &countingMeasurer{})

		if got := badge.TextWidth("", "Width Test G", 10, false); got != 0 {
			t.Fatalf("TextWidth() = %d, want 0", got)
		}
	})

	t.Run("concurrent draws are safe", func(t *testing.T) {
		installMeasurer(t, &countingMeasurer{})

		var group sync.WaitGroup

		for range 8 {
			group.Go(func() {
				for range 200 {
					if got := badge.TextWidth("WAW", "Width Test H", 10, false); got != 25 {
						t.Errorf("TextWidth() = %d, want 25", got)

						return
					}
				}
			})
		}

		group.Wait()
	})
}
