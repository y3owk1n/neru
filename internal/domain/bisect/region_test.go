package bisect_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/bisect"
)

var screen = image.Rect(0, 0, 1000, 800)

func TestRegion_Apply_KeepsTheNamedHalf(t *testing.T) {
	tests := []struct {
		cut  bisect.Cut
		want image.Rectangle
	}{
		{bisect.CutLeft, image.Rect(0, 0, 500, 800)},
		{bisect.CutRight, image.Rect(500, 0, 1000, 800)},
		{bisect.CutUp, image.Rect(0, 0, 1000, 400)},
		{bisect.CutDown, image.Rect(0, 400, 1000, 800)},
		{bisect.CutUpLeft, image.Rect(0, 0, 500, 400)},
		{bisect.CutDownRight, image.Rect(500, 400, 1000, 800)},
	}

	for _, testCase := range tests {
		t.Run(testCase.cut.String(), func(t *testing.T) {
			region := bisect.NewRegion(screen)

			if !region.Apply(testCase.cut) {
				t.Fatal("a cut on a whole screen must change the region")
			}

			if got := region.Bounds(); !got.Eq(testCase.want) {
				t.Fatalf("bounds = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestRegion_Apply_ConvergesAndCenterStaysInside(t *testing.T) {
	region := bisect.NewRegion(screen)

	for range 12 {
		region.Apply(bisect.CutDownRight)
	}

	got := region.Bounds()
	if got.Dx() > bisect.MinSide || got.Dy() > bisect.MinSide {
		t.Fatalf("twelve quadrant cuts left %v, want a region at MinSide", got)
	}

	if !region.Center().In(got) {
		t.Fatalf("center %v is outside %v", region.Center(), got)
	}
}

func TestRegion_Apply_RefusesACutThatChangesNothing(t *testing.T) {
	region := bisect.NewRegion(image.Rect(0, 0, 2, 800))

	// The width is already at MinSide: a horizontal cut is refused whole,
	// and costs no history, while a quadrant cut still takes the height.
	if region.Apply(bisect.CutLeft) || region.Depth() != 0 {
		t.Fatal("a cut on a finished axis must be refused")
	}

	if !region.Apply(bisect.CutUpLeft) || region.Bounds().Dy() != 400 {
		t.Fatalf("a quadrant cut must still take the unfinished axis: %v", region.Bounds())
	}
}

func TestRegion_ApplyTimes_RepeatsTheCutAsOneHistoryEntry(t *testing.T) {
	region := bisect.NewRegion(screen)

	if !region.ApplyTimes(bisect.CutLeft, 2) {
		t.Fatal("two left cuts on a whole screen must change the region")
	}

	if got := region.Bounds(); !got.Eq(image.Rect(0, 0, 250, 800)) {
		t.Fatalf("bounds = %v, want the left quarter", got)
	}

	if region.Depth() != 1 || !region.Backtrack() || !region.Bounds().Eq(screen) {
		t.Fatalf(
			"a counted press must undo as one: depth %d, bounds %v",
			region.Depth(),
			region.Bounds(),
		)
	}
}

func TestRegion_ApplyTimes_KeepsTheCutsThatFit(t *testing.T) {
	// Width 8 fits two halvings to MinSide. The third changes nothing and
	// stops the run without undoing the two before it.
	region := bisect.NewRegion(image.Rect(0, 0, 8, 800))

	if !region.ApplyTimes(bisect.CutRight, 3) {
		t.Fatal("a run with cuts that fit must change the region")
	}

	if got := region.Bounds(); !got.Eq(image.Rect(6, 0, 8, 800)) || region.Depth() != 1 {
		t.Fatalf("bounds = %v at depth %d, want (6,0)-(8,800) at depth 1", got, region.Depth())
	}

	if region.ApplyTimes(bisect.CutRight, 3) || region.ApplyTimes(bisect.CutLeft, 0) {
		t.Fatal("a run in which no cut fits, or a count below one, must be refused")
	}
}

func TestRegion_Apply_RefusesACutThatWouldFallBelowMinSide(t *testing.T) {
	// Width 3 is above MinSide, but its low half is one pixel.
	region := bisect.NewRegion(image.Rect(0, 0, 3, 800))

	if region.Apply(bisect.CutLeft) || region.Apply(bisect.CutRight) {
		t.Fatalf("a cut on a three pixel axis must be refused, got %v", region.Bounds())
	}

	region = bisect.NewRegion(image.Rect(0, 0, 1920, 800))
	region.ApplyTimes(bisect.CutLeft, 20)

	if got := region.Bounds().Dx(); got < bisect.MinSide {
		t.Fatalf("repeated left cuts left width %d, below MinSide", got)
	}
}

func TestRegion_BacktrackAndReset(t *testing.T) {
	region := bisect.NewRegion(screen)

	if region.Backtrack() {
		t.Fatal("backtrack with no history must report nothing to do")
	}

	region.Apply(bisect.CutLeft)
	region.Apply(bisect.CutUp)

	if !region.Backtrack() || !region.Bounds().Eq(image.Rect(0, 0, 500, 800)) {
		t.Fatalf("backtrack left %v", region.Bounds())
	}

	region.Reset()

	if !region.Bounds().Eq(screen) || region.Depth() != 0 {
		t.Fatalf("reset left %v at depth %d", region.Bounds(), region.Depth())
	}
}

func TestRegion_RemapToNewBounds_ScalesRegionAndHistory(t *testing.T) {
	region := bisect.NewRegion(screen)
	region.Apply(bisect.CutRight)
	region.Apply(bisect.CutDown)

	region.RemapToNewBounds(image.Rect(0, 0, 2000, 1600))

	if got := region.Bounds(); !got.Eq(image.Rect(1000, 800, 2000, 1600)) {
		t.Fatalf("remapped region = %v", got)
	}

	region.Backtrack()

	if got := region.Bounds(); !got.Eq(image.Rect(1000, 0, 2000, 1600)) {
		t.Fatalf("remapped history = %v", got)
	}
}

func TestRegion_RemapToNewBounds_KeepsANarrowRegionDrawable(t *testing.T) {
	// The rightmost two pixels of an 8K-wide display land on the last pixel
	// of a 1920-wide one and would round to nothing.
	region := bisect.NewRegion(image.Rect(0, 0, 7680, 1080))
	for range 12 {
		region.Apply(bisect.CutRight)
	}

	if got := region.Bounds(); got.Dx() != bisect.MinSide {
		t.Fatalf("twelve right cuts left width %d, want MinSide", got.Dx())
	}

	region.RemapToNewBounds(image.Rect(0, 0, 1920, 1080))

	got := region.Bounds()
	if got.Empty() || got.Dx() < bisect.MinSide || got.Max.X > 1920 {
		t.Fatalf("remapped region = %v, want at least MinSide wide inside the display", got)
	}

	if !region.Center().In(image.Rect(0, 0, 1920, 1080)) {
		t.Fatalf("center %v is off the display", region.Center())
	}
}

func TestParseCut_AcceptsEverySpellingAndRefusesTheRest(t *testing.T) {
	for index, name := range bisect.CutNames {
		cut, err := bisect.ParseCut(name)
		if err != nil || int(cut) != index {
			t.Fatalf("ParseCut(%q) = %v, %v", name, cut, err)
		}
	}

	cut, err := bisect.ParseCut("Down-Left")
	if err != nil || cut != bisect.CutDownLeft {
		t.Fatalf("hyphenated, mixed-case spelling answered %v, %v", cut, err)
	}

	_, err = bisect.ParseCut("sideways")
	if err == nil {
		t.Fatal("an unknown cut was accepted")
	}
}
