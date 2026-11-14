package badge

import (
	"image"
	"math"
	"testing"
)

// alphaAt reads the mask at a point given relative to the badge's top-left
// corner, answering 0 off the mask.
func alphaAt(mask *ShadowMask, point image.Point) int {
	local := point.Sub(mask.Origin)
	if local.X < 0 || local.Y < 0 || local.X >= mask.Width || local.Y >= mask.Height {
		return 0
	}

	return int(mask.Alpha[local.Y*mask.Width+local.X])
}

func TestShadowCache_HintShadow_ZeroBlurIsTheOutline(t *testing.T) {
	t.Parallel()

	var cache ShadowCache

	mask := cache.HintShadow(image.Rect(10, 20, 40, 34), 0, HintArrow{}, false, 0)

	if mask.Origin != (image.Point{}) || mask.Width != 30 || mask.Height != 14 {
		t.Fatalf("mask = origin %v size %dx%d, want the badge's own 30x14 at its corner",
			mask.Origin, mask.Width, mask.Height)
	}

	for _, point := range []image.Point{{0, 0}, {15, 7}, {29, 13}} {
		if got := alphaAt(mask, point); got != maxAlpha {
			t.Errorf("alpha at %v = %d, want %d inside the badge", point, got, maxAlpha)
		}
	}
}

// A badge far wider and taller than its blur has edges that blur like a
// half-plane, whose profile is the Gaussian's tail: half the erfc of the
// distance over sigma*sqrt(2). That is the shadow a browser draws for the
// same CSS blur, so this pins the meaning of shadow_blur.
func TestShadowCache_HintShadow_EdgeFollowsTheCSSGaussian(t *testing.T) {
	t.Parallel()

	const (
		blur      = 14.0
		tolerance = 2
	)

	var cache ShadowCache

	mask := cache.HintShadow(image.Rect(0, 0, 200, 120), 0, HintArrow{}, false, blur)
	sigma := blur * shadowSigmaPerBlur

	for distance := range 20 {
		// The pixel whose center lies distance+0.5 above the top edge.
		point := image.Pt(100, -distance-1)
		want := maxAlpha * 0.5 * math.Erfc((float64(distance)+pixelCenter)/(sigma*math.Sqrt2))

		if got := alphaAt(mask, point); math.Abs(float64(got)-want) > tolerance {
			t.Errorf("alpha %d px above the edge = %d, want %.1f", distance, got, want)
		}
	}
}

func TestShadowCache_HintShadow_KeepsTheOutlinesWeight(t *testing.T) {
	t.Parallel()

	var cache ShadowCache

	sharp := cache.HintShadow(image.Rect(0, 0, 24, 14), 3, HintArrow{}, false, 0)
	soft := cache.HintShadow(image.Rect(0, 0, 24, 14), 3, HintArrow{}, false, 7)

	sum := func(mask *ShadowMask) int {
		total := 0
		for _, alpha := range mask.Alpha {
			total += int(alpha)
		}

		return total
	}

	// Blurring spreads the shadow and loses none of it, short of rounding.
	if got, want := sum(soft), sum(sharp); math.Abs(float64(got-want)) > float64(want)/50 {
		t.Errorf("blurred mask holds %d, want about the outline's %d", got, want)
	}

	if soft.Width <= sharp.Width || soft.Height <= sharp.Height {
		t.Errorf("blurred mask is %dx%d, want it larger than the outline's %dx%d",
			soft.Width, soft.Height, sharp.Width, sharp.Height)
	}
}

func TestShadowCache_HintShadow_CoversTheArrow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		offset HintOffset
		inside image.Point // a point in the arrow, relative to the badge corner
	}{
		{name: "badge above its target", offset: HintAbove, inside: image.Pt(20, 16)},
		{name: "badge below its target", offset: HintBelow, inside: image.Pt(20, -2)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			badgeRect, arrow, ok := PlaceHint(image.Pt(100, 100), 40, 14, 3, test.offset)
			if !ok {
				t.Fatal("PlaceHint gave no arrow")
			}

			var cache ShadowCache

			withArrow := cache.HintShadow(badgeRect, 3, arrow, true, 0)
			without := cache.HintShadow(badgeRect, 3, arrow, false, 0)

			if got := alphaAt(withArrow, test.inside); got != maxAlpha {
				t.Errorf("alpha in the arrow at %v = %d, want %d", test.inside, got, maxAlpha)
			}

			if got := alphaAt(without, test.inside); got != 0 {
				t.Errorf("alpha at %v without the arrow = %d, want 0", test.inside, got)
			}
		})
	}
}

func TestShadowCache_HintShadow_ReusesTheMaskOfTheSameShape(t *testing.T) {
	t.Parallel()

	var cache ShadowCache

	first := cache.HintShadow(image.Rect(0, 0, 30, 14), 3, HintArrow{}, false, 7)
	moved := cache.HintShadow(image.Rect(500, 300, 530, 314), 3, HintArrow{}, false, 7)
	wider := cache.HintShadow(image.Rect(0, 0, 31, 14), 3, HintArrow{}, false, 7)

	if first != moved {
		t.Error("a badge of the same shape elsewhere built a second mask, want the cached one")
	}

	if first == wider {
		t.Error("a wider badge reused the narrower one's mask")
	}
}

func TestShadowCache_HintShadow_StaysBounded(t *testing.T) {
	t.Parallel()

	var cache ShadowCache

	for width := 1; width <= maxCachedShadows*2; width++ {
		cache.HintShadow(image.Rect(0, 0, width, 10), 0, HintArrow{}, false, 2)
	}

	if len(cache.masks) > maxCachedShadows {
		t.Errorf("cache holds %d masks, want at most %d", len(cache.masks), maxCachedShadows)
	}
}

func TestShadowMask_Rect_AppliesTheBadgeCornerAndTheOffset(t *testing.T) {
	t.Parallel()

	mask := &ShadowMask{Origin: image.Pt(-11, -11), Width: 52, Height: 36}

	got := mask.Rect(image.Pt(100, 50), image.Pt(0, 3))
	want := image.Rect(89, 42, 141, 78)

	if got != want {
		t.Errorf("Rect = %v, want %v", got, want)
	}
}
