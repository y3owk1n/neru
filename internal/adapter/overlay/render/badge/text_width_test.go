package badge_test

import (
	"strconv"
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

	t.Run("a face is a family and a weight, whatever the size", func(t *testing.T) {
		measurer := &countingMeasurer{}
		installMeasurer(t, measurer)

		// One table serves every size and every display scale, which is what
		// lets it be filled before the scale is known.
		if at10, at20 := badge.TextWidth("A", "Width Test C", 10, false),
			badge.TextWidth("A", "Width Test C", 20, false); at10 != 5 || at20 != 10 {
			t.Fatalf("TextWidth() = %d at 10 and %d at 20, want 5 and 10", at10, at20)
		}

		badge.TextWidth("A", "Width Test C", 10, true)
		badge.TextWidth("A", "Width Test D", 10, false)

		if asked := measurer.asked(); asked != 3 {
			t.Fatalf("measured %d times for 3 faces, want 3", asked)
		}
	})

	t.Run("a warmed face measures nothing when it is drawn", func(t *testing.T) {
		measurer := &countingMeasurer{}
		installMeasurer(t, measurer)

		badge.WarmTextWidths("Width Test W", true, "●")

		warmed := measurer.asked()
		if warmed == 0 {
			t.Fatal("WarmTextWidths measured nothing")
		}

		for _, label := range []string{"AS", "/ sav  3", "Scroll", "WW", "~ !"} {
			badge.TextWidth(label, "Width Test W", 12, true)
		}

		if asked := measurer.asked(); asked != warmed {
			t.Fatalf("a draw measured %d more times after warming, want none", asked-warmed)
		}
	})

	t.Run("warming a face the platform cannot measure does nothing", func(t *testing.T) {
		installMeasurer(t, nil)

		badge.WarmTextWidths("Width Test X", false, "")

		if got, want := badge.TextWidth("AB", "Width Test X", 10, false),
			badge.EstimateTextWidth("AB", 10); got != want {
			t.Fatalf("TextWidth() = %d, want the estimate %d", got, want)
		}
	})

	t.Run("the tables kept are bounded", func(t *testing.T) {
		measurer := &countingMeasurer{}
		installMeasurer(t, measurer)

		for index := range 100 {
			badge.TextWidth("A", "Width Test Bound "+strconv.Itoa(index), 10, false)
		}

		// Far past the bound, the first face is measured again, not remembered
		// for ever.
		before := measurer.asked()

		badge.TextWidth("A", "Width Test Bound 0", 10, false)

		if measurer.asked() == before {
			t.Fatal("a face from 100 faces ago was still held")
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
