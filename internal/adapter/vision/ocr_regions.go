package vision

import (
	"fmt"
	"image"
	"strings"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// recognizedWord is one run of text an OCR engine reported.
//
// Bounds are in the captured image's own pixel space — origin (0, 0), which is
// the top-left of the *captured region* rather than of the screen — because
// that is the only space an engine handed a buffer of pixels can answer in.
// regionsFromWords is what puts them back on the screen.
//
// Text is screen content. It reaches an element's title and search text, and it
// is never logged, counted into a log message, or written anywhere else.
type recognizedWord struct {
	Text   string
	Bounds image.Rectangle
	// Confidence is 0..1, matching config's minimum_confidence and the score
	// the heuristic classifier compares against. An engine that reports
	// percentages is normalized before it gets here.
	Confidence float64
}

// regionsFromWords maps recognized words onto Neru's shared coordinate space
// and drops the findings that are not worth a hint.
//
// region is what the caller asked to have captured, in global top-left-origin
// unscaled pixels. imageBounds is what the capture backend actually returned,
// which is *not* always the same size: a Wayland compositor answers a region
// request in physical pixels, so a 2x output hands back a frame twice the
// requested size (screencapture_common.go documents this). The ratio between
// the two is the output scale, and dividing by it is what puts a hint on its
// text rather than a quarter of the way up the window.
//
// minConfidence is inclusive: a word scoring exactly the configured floor is
// kept, so a user who writes the number they observed keeps the word they wrote
// it for.
//
// A region or an image with no area has no mapping, so nothing is returned
// rather than boxes in the wrong space.
func regionsFromWords(
	words []recognizedWord,
	region image.Rectangle,
	imageBounds image.Rectangle,
	minConfidence float64,
) []DetectedRegion {
	if region.Empty() || imageBounds.Empty() {
		return nil
	}

	scaleX := float64(imageBounds.Dx()) / float64(region.Dx())
	scaleY := float64(imageBounds.Dy()) / float64(region.Dy())

	regions := make([]DetectedRegion, 0, len(words))

	for _, word := range words {
		if strings.TrimSpace(word.Text) == "" {
			continue
		}

		if word.Confidence < minConfidence {
			continue
		}

		bounds := image.Rect(
			region.Min.X+int(float64(word.Bounds.Min.X)/scaleX),
			region.Min.Y+int(float64(word.Bounds.Min.Y)/scaleY),
			region.Min.X+int(float64(word.Bounds.Max.X)/scaleX),
			region.Min.Y+int(float64(word.Bounds.Max.Y)/scaleY),
		).Intersect(region)

		if bounds.Empty() {
			continue
		}

		regions = append(regions, DetectedRegion{
			Bounds: bounds,
			Label:  word.Text,
			Score:  word.Confidence,
			IsText: true,
		})
	}

	return regions
}

// elementsFromRegions turns classified regions into domain elements. The second
// result counts the regions that produced none, so a caller can log how many
// were dropped without logging anything about them — a vision region's label is
// text off the user's screen.
//
// Both backends share this: the classification differs by vocabulary, the
// element it becomes does not.
func elementsFromRegions(
	regions []DetectedRegion,
	classifier *regionClassifier,
) ([]*element.Element, int) {
	elements := make([]*element.Element, 0, len(regions))
	skipped := 0

	for _, region := range regions {
		if region.Bounds.Empty() {
			skipped++

			continue
		}

		role, clickable := classifier.Classify(region)

		opts := []element.Option{element.WithVisionOnly()}
		if clickable {
			opts = append(opts, element.WithClickable(true))
		}

		if region.Label != "" {
			opts = append(opts,
				element.WithTitle(region.Label),
				element.WithSearchText(region.Label),
			)
		}

		elem, err := element.NewElement(
			element.ID("vision-"+regionBoundsKey(region.Bounds)),
			region.Bounds,
			element.Role(role),
			opts...,
		)
		if err != nil {
			skipped++

			continue
		}

		elements = append(elements, elem)
	}

	return elements, skipped
}

// regionBoundsKey returns a deterministic string key for a rectangle, used as
// the element ID for vision-detected elements. The same control found again on
// the next refresh keeps the same hint identity.
func regionBoundsKey(r image.Rectangle) string {
	return fmt.Sprintf("%d-%d-%d-%d", r.Min.X, r.Min.Y, r.Max.X, r.Max.Y)
}
