//go:build integration && linux && cgo

package linux

import (
	"image"
	"math"
	"os"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/config"
)

// probeNotchPixels is what this test calls one wheel notch. It is stated here
// rather than taken from the package so the measurement does not inherit the
// conversion it is checking; TestScrollAtCursor_ConvertsPixelsToNotchesTheWayTheProbeExpects
// fails if the two ever disagree.
const probeNotchPixels = 30

// axisSourceContinuous is wl_pointer.axis_source's `continuous` member, spelled
// out here rather than imported for the same reason: this is the independent
// statement of what has to arrive on the wire.
const axisSourceContinuous = 2

// fixedConfig is a config.Provider over one snapshot, which is all the scroll
// path reads.
type fixedConfig struct{ cfg *config.Config }

func (f fixedConfig) Get() *config.Config { return f.cfg }

// requireDesktop skips unless this run opted into tests that drive the real
// desktop. `just test-desktop` sets the variable, and the headless-sway CI leg
// is what runs it; plain `just test` stays hands-off the machine.
func requireDesktop(t *testing.T) {
	t.Helper()

	if os.Getenv("NERU_DESKTOP_TESTS") == "" {
		t.Skip("skipping desktop-driving test; run `just test-desktop` to include it")
	}

	if os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("skipping: no Wayland session; this measures the wlroots injection path")
	}
}

// TestScrollAtCursor_ConvertsPixelsToNotchesTheWayTheProbeExpects keeps the
// measurement honest: the two tests below read a delivered value against
// probeNotchPixels, which means nothing if the injection path scales by
// something else.
func TestScrollAtCursor_ConvertsPixelsToNotchesTheWayTheProbeExpects(t *testing.T) {
	if scrollPixelsPerNotch != probeNotchPixels {
		t.Fatalf("the scroll path converts %d pixels to a notch, the probe assumes %d",
			scrollPixelsPerNotch, probeNotchPixels)
	}
}

// startProbe maps the probe window for one test.
func startProbe(t *testing.T) *scrollProbe {
	t.Helper()

	probe, ok := startScrollProbe()
	if !ok {
		t.Skip("skipping: could not map a Wayland window in this session")
	}

	t.Cleanup(probe.stop)

	return probe
}

// pump dispatches for a while, so events the compositor sends while the
// animation runs are read as they arrive rather than all at the end.
func pump(probe *scrollProbe, d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		probe.dispatch(20)
	}
}

// aimAtTheProbe parks the pointer over the probe window and waits for the
// compositor to say so, because a client with no pointer focus receives no
// scroll however it is injected.
func aimAtTheProbe(t *testing.T, probe *scrollProbe) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {
		MoveMouseToPoint(image.Point{X: 400, Y: 400}, false)

		pump(probe, 100*time.Millisecond)

		if probe.entered() {
			return
		}
	}

	t.Fatal("the pointer never entered the probe window; nothing could be measured")
}

// TestScrollAtCursor_DeliversSubNotchStepsWithSmoothScroll is the measurement
// the smooth-scroll work rests on: with smooth_scroll.enabled, one scroll
// reaches a real application as a sequence of continuous deltas, at least one
// of which is shorter than a wheel notch and none of which carries a notch
// count.
//
// That is the question X11 answers no to and Wayland answers yes to. If the
// compositor quantised the animation back to notches, every delivered event
// would be a whole notch and the animation would be no finer than the scroll it
// replaced.
func TestScrollAtCursor_DeliversSubNotchStepsWithSmoothScroll(t *testing.T) {
	requireDesktop(t)

	probe := startProbe(t)
	aimAtTheProbe(t, probe)

	cfg := config.DefaultConfig()
	cfg.SmoothScroll.Enabled = true
	cfg.SmoothScroll.Steps = 16
	cfg.SmoothScroll.MaxDuration = 200
	cfg.SmoothScroll.DurationPerPixel = 1.0

	SetConfigProvider(fixedConfig{cfg: cfg})
	t.Cleanup(func() { SetConfigProvider(nil) })

	// One notch's worth of scroll over sixteen steps: every step is a
	// sixteenth of a notch, so a backend that could only move in notches would
	// deliver one event, or none.
	err := ScrollAtCursor(0, -probeNotchPixels, 0)
	if err != nil {
		t.Fatalf("ScrollAtCursor with smooth scroll on failed: %v", err)
	}

	pump(probe, 3*time.Second)

	delivered := probe.axisEvents()
	if len(delivered) < 2 {
		t.Fatalf("the probe received %d scroll events; an animated scroll arrives as several",
			len(delivered))
	}

	var (
		subNotch int
		total    float64
	)

	for _, event := range delivered {
		total += math.Abs(event.value)

		if event.hasStep {
			t.Errorf("a delivered event carried a wheel-step count (%v); "+
				"an animated step is a distance, not a notch", event.value)
		}

		// A wheel source would tell the application the value came off a
		// detented device, which invites it to round the fraction back to a
		// detent whatever the protocol carried.
		if event.source != axisSourceContinuous {
			t.Errorf("a delivered event declared axis source %d, want %d (continuous)",
				event.source, axisSourceContinuous)
		}

		if event.value != 0 && math.Abs(event.value) < probeNotchPixels {
			subNotch++
		}
	}

	if subNotch == 0 {
		t.Fatalf("every delivered event was a whole notch or more (%d events, %v total); "+
			"the sub-notch deltas did not survive the compositor", len(delivered), total)
	}
}

// TestScrollAtCursor_DeliversWholeNotchesWithoutSmoothScroll is the control.
// The same scroll with the animation off arrives as the notch it always was,
// which is what makes the test above a measurement of smooth scroll rather than
// of the session it runs in.
func TestScrollAtCursor_DeliversWholeNotchesWithoutSmoothScroll(t *testing.T) {
	requireDesktop(t)

	probe := startProbe(t)
	aimAtTheProbe(t, probe)

	cfg := config.DefaultConfig()
	cfg.SmoothScroll.Enabled = false

	SetConfigProvider(fixedConfig{cfg: cfg})
	t.Cleanup(func() { SetConfigProvider(nil) })

	err := ScrollAtCursor(0, -probeNotchPixels, 0)
	if err != nil {
		t.Fatalf("ScrollAtCursor with smooth scroll off failed: %v", err)
	}

	pump(probe, 2*time.Second)

	delivered := probe.axisEvents()
	if len(delivered) == 0 {
		t.Fatal("the probe received no scroll at all; the session cannot inject")
	}

	for _, event := range delivered {
		if math.Abs(event.value) < probeNotchPixels {
			t.Errorf("an unanimated scroll delivered %v, less than one notch", event.value)
		}
	}
}
