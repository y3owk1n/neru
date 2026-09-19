//go:build integration && darwin

package darwin_test

import (
	"math"
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	"github.com/y3owk1n/neru/internal/ports"
)

// measureOrFail measures and fails the test on an error, so each case reads as
// the claim it makes.
func measureOrFail(t *testing.T, text, family string, size float64, bold bool) ports.TextMetrics {
	t.Helper()

	metrics, err := darwin.NewTextMeasurer().Measure(text, family, size, bold)
	if err != nil {
		t.Fatalf("Measure(%q, %q, %v, %v) error = %v", text, family, size, bold, err)
	}

	if metrics.Width <= 0 || metrics.Height <= 0 {
		t.Fatalf(
			"Measure(%q, %q, %v, %v) = %+v, want a positive size",
			text,
			family,
			size,
			bold,
			metrics,
		)
	}

	return metrics
}

func TestTextMeasurer_Measure_ScalesLinearlyWithFontSize(t *testing.T) {
	// The fit policy measures once at the configured size and derives every
	// smaller size by arithmetic. That is only sound if this holds.
	const tolerance = 0.12

	for _, family := range []string{"Helvetica Neue", "Menlo"} {
		t.Run(family, func(t *testing.T) {
			large := measureOrFail(t, "WM", family, 40, false)
			small := measureOrFail(t, "WM", family, 10, false)

			for name, ratio := range map[string]float64{
				"width":  large.Width / small.Width,
				"height": large.Height / small.Height,
			} {
				if math.Abs(ratio-4)/4 > tolerance {
					t.Errorf(
						"%s grew %.2fx from size 10 to 40, want 4x within %.0f%%",
						name,
						ratio,
						tolerance*100,
					)
				}
			}
		})
	}
}

func TestTextMeasurer_Measure_LongerTextIsWider(t *testing.T) {
	one := measureOrFail(t, "W", "Helvetica Neue", 20, false)
	three := measureOrFail(t, "WWW", "Helvetica Neue", 20, false)

	if three.Width <= one.Width {
		t.Fatalf(
			"\"WWW\" is %.1f wide and \"W\" is %.1f, want the longer string wider",
			three.Width,
			one.Width,
		)
	}

	if three.Height != one.Height {
		t.Fatalf(
			"heights differ (%.1f, %.1f), want the line height of one font",
			three.Height,
			one.Height,
		)
	}
}

func TestTextMeasurer_Measure_MissingFamilyStillMeasures(t *testing.T) {
	// The draw path substitutes a font for a family that is not installed, so
	// the label is drawn either way and has to be fitted either way.
	measureOrFail(t, "A", "Neru No Such Font Family 4f2a", 12, false)
	measureOrFail(t, "A", "", 12, true)
}

func TestTextMeasurer_Measure_IsSafeOffTheMainThread(t *testing.T) {
	// Styles are rebuilt on background goroutines at reload and theme change.
	var group sync.WaitGroup

	for range 8 {
		group.Go(func() {
			for size := 8; size < 40; size++ {
				_, err := darwin.NewTextMeasurer().
					Measure("AB", "Helvetica Neue", float64(size), false)
				if err != nil {
					t.Errorf("Measure() at size %d error = %v", size, err)

					return
				}
			}
		})
	}

	group.Wait()
}
