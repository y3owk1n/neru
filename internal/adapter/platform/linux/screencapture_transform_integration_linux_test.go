//go:build integration && linux

package linux

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// quadrantColors is the background the transform test paints, one color per
// corner, so any turn or mirror of the captured frame moves a color to the
// wrong corner.
var quadrantColors = [4]color.RGBA{
	{R: 0xFF, A: 0xFF},                   // top left
	{G: 0xFF, A: 0xFF},                   // top right
	{B: 0xFF, A: 0xFF},                   // bottom left
	{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, // bottom right
}

// TestCaptureScreenRegion_ReadsATransformedOutputAsShown rotates and mirrors
// the output and checks that every corner of the captured frame holds the
// color painted there. Screencopy answers in the output's buffer orientation,
// so without undoing the transform a rotated output's frame came back turned.
//
// It rotates the output and replaces its background, so it runs only when
// desktop tests were asked for, and only on a headless sway output: a real
// monitor's wallpaper cannot be read back to restore it.
func TestCaptureScreenRegion_ReadsATransformedOutputAsShown(t *testing.T) {
	if testing.Short() || os.Getenv("NERU_DESKTOP_TESTS") == "" {
		t.Skip("skipping desktop-driving test; run `just test-desktop` to include it")
	}

	if os.Getenv("WAYLAND_DISPLAY") == "" || os.Getenv("SWAYSOCK") == "" {
		t.Skip("needs a sway session: set WAYLAND_DISPLAY and SWAYSOCK")
	}

	swaymsg, err := exec.LookPath("swaymsg")
	if err != nil {
		t.Skip("swaymsg is not installed")
	}

	original := swayOutput(t, swaymsg)
	if !strings.HasPrefix(original.name, "HEADLESS-") {
		t.Skipf("output %s is a real monitor; this test only rotates a headless one", original.name)
	}

	background := writeQuadrantBackground(t)

	t.Cleanup(func() {
		_ = exec.CommandContext(
			context.Background(), swaymsg, "output", original.name, "transform", original.transform,
		).Run()
	})

	runSwaymsg(t, swaymsg, "output", original.name, "bg", background, "stretch")

	for _, transform := range []string{
		"normal", "90", "180", "270", "flipped", "flipped-90", "flipped-180", "flipped-270",
	} {
		t.Run(transform, func(t *testing.T) {
			runSwaymsg(t, swaymsg, "output", original.name, "transform", transform)

			// A quarter turn swaps the output's logical width and height, so the
			// test reads the region back from sway after each change.
			region := swayOutput(t, swaymsg).rect

			var got [4]color.RGBA

			deadline := time.Now().Add(5 * time.Second)
			for {
				img := requireCapture(t, backendWaylandWlroots, region)
				got = quadrantSamples(img)

				if got == quadrantColors || time.Now().After(deadline) {
					break
				}

				// swaybg redraws after the output changes, so give it time.
				time.Sleep(100 * time.Millisecond)
			}

			// A frame turned the wrong way still holds the painted colors, in
			// the wrong corners. A frame with none of them means the compositor
			// drew no background for this transform, which sway 1.9 does for the
			// mirrored ones, and says nothing about the capture.
			if !holdsAnyQuadrantColor(got) {
				t.Skipf("the compositor drew no background for transform %s", transform)
			}

			if got != quadrantColors {
				t.Errorf("corners (top left, top right, bottom left, bottom right) = %v, want %v",
					got, quadrantColors)
			}
		})
	}
}

// holdsAnyQuadrantColor reports whether any corner sample is one of the painted
// colors.
func holdsAnyQuadrantColor(samples [4]color.RGBA) bool {
	for _, sample := range samples {
		if slices.Contains(quadrantColors[:], sample) {
			return true
		}
	}

	return false
}

// quadrantSamples reads the middle of each quarter of img.
func quadrantSamples(img *image.RGBA) [4]color.RGBA {
	width := img.Rect.Dx()
	height := img.Rect.Dy()

	sample := func(col, row int) color.RGBA {
		return img.RGBAAt(img.Rect.Min.X+col, img.Rect.Min.Y+row)
	}

	return [4]color.RGBA{
		sample(width/4, height/4),
		sample(width*3/4, height/4),
		sample(width/4, height*3/4),
		sample(width*3/4, height*3/4),
	}
}

// writeQuadrantBackground writes quadrantColors as a PNG for swaybg.
func writeQuadrantBackground(t *testing.T) string {
	t.Helper()

	const side = 200

	img := image.NewRGBA(image.Rect(0, 0, side, side))
	for row := range side {
		for col := range side {
			quadrant := 0
			if col >= side/2 {
				quadrant++
			}

			if row >= side/2 {
				quadrant += 2
			}

			img.SetRGBA(col, row, quadrantColors[quadrant])
		}
	}

	path := filepath.Join(t.TempDir(), "quadrants.png")

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create the background: %v", err)
	}

	defer func() { _ = file.Close() }()

	err = png.Encode(file, img)
	if err != nil {
		t.Fatalf("encode the background: %v", err)
	}

	return path
}

// swayOutputState is what the transform test reads about an output.
type swayOutputState struct {
	name      string
	transform string
	rect      image.Rectangle
}

// swayOutput returns the first output sway reports.
func swayOutput(t *testing.T, swaymsg string) swayOutputState {
	t.Helper()

	raw, err := exec.CommandContext(context.Background(), swaymsg, "-t", "get_outputs", "-r").
		Output()
	if err != nil {
		t.Fatalf("swaymsg -t get_outputs: %v", err)
	}

	var outputs []struct {
		Name      string `json:"name"`
		Transform string `json:"transform"`
		Rect      struct {
			X      int `json:"x"`
			Y      int `json:"y"`
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"rect"`
	}

	err = json.Unmarshal(raw, &outputs)
	if err != nil || len(outputs) == 0 {
		t.Fatalf("read sway's outputs from %q: %v", raw, err)
	}

	rect := outputs[0].Rect

	return swayOutputState{
		name:      outputs[0].Name,
		transform: outputs[0].Transform,
		rect:      image.Rect(rect.X, rect.Y, rect.X+rect.Width, rect.Y+rect.Height),
	}
}

func runSwaymsg(t *testing.T, swaymsg string, args ...string) {
	t.Helper()

	out, err := exec.CommandContext(context.Background(), swaymsg, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("swaymsg %v: %v (%s)", args, err, out)
	}
}
