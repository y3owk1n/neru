package segment_test

import (
	"context"
	"encoding/json"
	"image"
	"image/draw"
	"image/png"
	"os"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/vision/segment"
)

// TestDetect_FindsEveryTargetInRenderedScenes runs the detector over real
// browser renderings, each cropped to the part of a scene that pins a
// behavior: links inside paragraphs (article), faint borders and tiny row
// menus (table), and a dark chat with avatars, colored links and reactions
// (chat). The scenes were rendered by WebKit at 2x and scaled to 1x, the way
// a screen capture arrives, and every clickable DOM element in them is
// labeled. Each must get a hint whose click point, the box center, lands on
// it.
func TestDetect_FindsEveryTargetInRenderedScenes(t *testing.T) {
	t.Parallel()

	// maxHints is each scene's hint count when it was recorded plus a quarter,
	// so a rule that floods a page with stray targets fails here. Plain words
	// get hints too, which is why the article, eight links in two paragraphs,
	// has far more hints than targets. Raise a ceiling deliberately or not at
	// all.
	scenes := map[string]int{
		"article": 42,
		"table":   68,
		"chat":    78,
	}

	for name, maxHints := range scenes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			img, targets := loadScene(t, name)

			rects, err := segment.Detect(context.Background(), img, 1)
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}

			for _, target := range targets {
				hit := false

				for _, r := range rects {
					if (image.Point{(r.Min.X + r.Max.X) / 2, (r.Min.Y + r.Max.Y) / 2}).In(target) {
						hit = true

						break
					}
				}

				if !hit {
					t.Errorf("no hint lands on target %v", target)
				}
			}

			if len(rects) > maxHints {
				t.Errorf("%d hints, want at most %d", len(rects), maxHints)
			}
		})
	}
}

func loadScene(t *testing.T, name string) (*image.RGBA, []image.Rectangle) {
	t.Helper()

	file, err := os.Open("testdata/" + name + ".png")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()

	src, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	img := image.NewRGBA(src.Bounds())
	draw.Draw(img, img.Bounds(), src, image.Point{}, draw.Src)

	data, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}

	var boxes [][4]int

	err = json.Unmarshal(data, &boxes)
	if err != nil {
		t.Fatal(err)
	}

	targets := make([]image.Rectangle, 0, len(boxes))
	for _, b := range boxes {
		targets = append(targets, image.Rect(b[0], b[1], b[0]+b[2], b[1]+b[3]))
	}

	return img, targets
}
