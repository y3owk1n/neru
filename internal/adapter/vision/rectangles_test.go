package vision

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"slices"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

func TestRectangleRegions(t *testing.T) {
	const (
		button = "button"
		thin   = "thin"
		icon   = "icon"
	)

	frame := image.NewRGBA(image.Rect(0, 0, 400, 300))
	draw.Draw(frame, frame.Rect, image.NewUniform(color.White), image.Point{}, draw.Src)

	// Each shape is an outlined box. Its center in global coordinates names it.
	region := image.Rect(1000, 500, 1400, 800)
	shapes := map[string]image.Rectangle{
		button: image.Rect(40, 40, 160, 80),
		thin:   image.Rect(300, 40, 310, 160),
		icon:   image.Rect(40, 200, 56, 216),
	}

	for _, shape := range shapes {
		draw.Draw(frame, shape, image.NewUniform(color.Black), image.Point{}, draw.Src)
		draw.Draw(frame, shape.Inset(2), image.NewUniform(color.Gray{Y: 200}), image.Point{},
			draw.Src)
	}

	tests := []struct {
		name string
		edit func(*config.HintsVisionConfig)
		want []string
	}{
		{
			"the defaults drop the thin box by aspect",
			func(*config.HintsVisionConfig) {},
			[]string{button, icon},
		},
		{
			"a lower aspect floor keeps it",
			func(c *config.HintsVisionConfig) { c.RectangleMinAspect = 0.05 },
			[]string{button, icon, thin},
		},
		{
			"a size floor drops the icon",
			func(c *config.HintsVisionConfig) { c.RectangleMinSize = 0.1 },
			[]string{button},
		},
		{
			"one candidate keeps the largest",
			func(c *config.HintsVisionConfig) { c.RectangleMaxCandidates = 1 },
			[]string{button},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.DefaultConfig().Hints.Vision
			test.edit(&cfg)

			regions, err := rectangleRegions(t.Context(), frame, region, cfg)
			if err != nil {
				t.Fatalf("rectangleRegions failed: %v", err)
			}

			var got []string

			for name, shape := range shapes {
				center := shape.Add(region.Min).Min.Add(shape.Size().Div(2))
				if slices.ContainsFunc(regions, func(found DetectedRegion) bool {
					return center.In(found.Bounds)
				}) {
					got = append(got, name)
				}
			}

			slices.Sort(got)

			if len(regions) != len(test.want) || !slices.Equal(got, test.want) {
				t.Errorf("found %v in %d regions, want %v", got, len(regions), test.want)
			}
		})
	}
}

func TestRectangleRegions_StopsWhenTheContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	frame := image.NewRGBA(image.Rect(0, 0, 400, 300))

	_, err := rectangleRegions(ctx, frame, frame.Rect, config.DefaultConfig().Hints.Vision)
	if err == nil {
		t.Error("rectangleRegions answered after its context was done")
	}
}

func TestWithoutTextOverlap(t *testing.T) {
	word := DetectedRegion{Bounds: image.Rect(10, 10, 110, 40), Label: "Save", IsText: true}

	tests := []struct {
		name string
		rect image.Rectangle
		want bool
	}{
		{"a box around the word is the word's target", image.Rect(5, 5, 115, 45), false},
		{"a large box holding the word is its own target", image.Rect(0, 0, 400, 200), true},
		{"a box away from text is kept", image.Rect(200, 200, 240, 240), true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kept := withoutTextOverlap(
				[]DetectedRegion{{Bounds: test.rect, Score: 1}},
				[]DetectedRegion{word},
				config.DefaultVisionMergeIOUThreshold,
			)

			if got := len(kept) == 1; got != test.want {
				t.Errorf("kept = %v, want %v", got, test.want)
			}
		})
	}
}

func TestAdapter_StartRectangles(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 400, 300))
	draw.Draw(frame, frame.Rect, image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(frame, image.Rect(40, 40, 160, 80), image.NewUniform(color.Black), image.Point{},
		draw.Src)

	adapter := &Adapter{logger: zap.NewNop()}

	tests := []struct {
		name   string
		detect bool
		want   int
	}{
		{"detect_rectangles off runs nothing", false, 0},
		{"detect_rectangles on finds the box", true, 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.DefaultConfig().Hints.Vision
			cfg.DetectRectangles = test.detect

			got, err := adapter.startRectangles(t.Context(), frame, frame.Rect, cfg)()
			if err != nil {
				t.Fatalf("startRectangles failed: %v", err)
			}

			if len(got) != test.want {
				t.Errorf("found %d rectangles, want %d", len(got), test.want)
			}
		})
	}
}

func TestAdapter_StartRectangles_ReportsAFailedPass(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	frame := image.NewRGBA(image.Rect(0, 0, 400, 300))
	adapter := &Adapter{logger: zap.NewNop()}

	_, err := adapter.startRectangles(ctx, frame, frame.Rect, config.DefaultConfig().Hints.Vision)()
	if err == nil {
		t.Error("a canceled rectangle pass reported no error")
	}
}
