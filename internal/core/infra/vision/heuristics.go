package vision

import (
	"image"
	"math"
)

// DetectedRegion represents a region of interest identified by a Vision
// Framework request. Multiple requests (text, rectangles, saliency) produce
// overlapping regions that are merged into a single element.
type DetectedRegion struct {
	Bounds image.Rectangle
	Label  string
	Score  float64
	IsText bool
}

// regionClassifier applies geometric and saliency heuristics to assign
// roles to detected regions. It is the first-pass classifier used on all
// platforms; a Core ML model can be swapped in later for improved accuracy.
type regionClassifier struct{}

const roleButton = "AXButton"

const buttonIconMaxSize = 48 // max dimension (px) for near-square regions to be considered buttons/icons

// Classify assigns a role to a detected region based on its geometry and
// saliency score. Returns the role string and whether the region is considered
// clickable.
func (c *regionClassifier) Classify(region DetectedRegion) (string, bool) {
	bounds := region.Bounds
	width := float64(bounds.Dx())
	height := float64(bounds.Dy())

	if width <= 0 || height <= 0 {
		return "AXUnknown", false
	}

	aspectRatio := width / height

	switch {
	case region.IsText && isLikelyButton(aspectRatio, region.Score, width, height):
		return roleButton, true
	case region.IsText && isLikelyLink(aspectRatio, width, height):
		return "AXLink", true
	case region.IsText:
		return "AXStaticText", false
	case isLikelyCheckBox(aspectRatio, width, height):
		return "AXCheckBox", true
	case isLikelyButton(aspectRatio, region.Score, width, height):
		return roleButton, true
	case isLikelyImage(aspectRatio, width, height):
		return "AXImage", false
	default:
		if region.Score > 0.5 { //nolint:mnd
			return roleButton, true
		}

		return "AXGenericElement", false
	}
}

// isLikelyButton checks aspect ratio and saliency to guess "button".
// Buttons and toolbar icons are typically 0.8:1 to 8:1 width:height
// (square-ish to wide) with decent saliency. For near-square regions,
// size is constrained to avoid catching large images.
func isLikelyButton(aspectRatio, score, width, height float64) bool {
	if aspectRatio < 0.8 || aspectRatio > 8.0 || score < 0.3 {
		return false
	}

	// Near-square regions: limit size to avoid classifying images as buttons
	if aspectRatio >= 0.8 && aspectRatio <= 1.5 {
		return max(width, height) <= buttonIconMaxSize
	}

	return true
}

// isLikelyLink checks for long, narrow text (links).
func isLikelyLink(aspectRatio, w, h float64) bool {
	return aspectRatio > 5.0 && h < 40 && w > 50
}

// isLikelyImage checks for near-square regions that are large enough to be
// actual images rather than buttons or icons. Images ≥ 48px in both dimensions
// with near-square aspect are classified as non-interactive.
func isLikelyImage(aspectRatio, w, h float64) bool {
	return aspectRatio >= 0.8 && aspectRatio <= 1.5 && w >= 48 && h >= 48
}

// isLikelyCheckBox checks for small square-ish regions (< 32px in both dims).
func isLikelyCheckBox(aspectRatio, w, h float64) bool {
	return aspectRatio >= 0.8 && aspectRatio <= 1.5 && w < 32 && h < 32
}

// MergeRegions merges overlapping regions using non-maximum suppression.
// Regions with higher scores suppress lower-scoring overlaps (IoU > 0.5).
func MergeRegions(regions []DetectedRegion) []DetectedRegion {
	if len(regions) == 0 {
		return nil
	}

	// Sort by score descending
	sorted := make([]DetectedRegion, len(regions))
	copy(sorted, regions)
	sortRegionsByScore(sorted)

	var result []DetectedRegion
	for len(sorted) > 0 {
		best := sorted[0]
		sorted = sorted[1:]

		var remaining []DetectedRegion
		for _, r := range sorted {
			iou := intersectionOverUnion(best.Bounds, r.Bounds)
			if iou < 0.5 { //nolint:mnd
				remaining = append(remaining, r)
			}
		}

		sorted = remaining

		result = append(result, best)
	}

	return result
}

// intersectionOverUnion computes the IoU of two rectangles.
func intersectionOverUnion(pointA, pointB image.Rectangle) float64 {
	intersection := pointA.Intersect(pointB)
	if intersection.Empty() {
		return 0
	}

	intersectArea := float64(intersection.Dx() * intersection.Dy())

	unionArea := float64(pointA.Dx()*pointA.Dy()) + float64(pointB.Dx()*pointB.Dy()) - intersectArea
	if unionArea <= 0 {
		return 0
	}

	return math.Round(intersectArea/unionArea*100) / 100 //nolint:mnd
}

// sortRegionsByScore sorts regions descending by Score using a simple
// insertion sort (n is typically very small, < 100).
func sortRegionsByScore(regions []DetectedRegion) {
	for i := 1; i < len(regions); i++ {
		key := regions[i]

		insertPos := i - 1
		for insertPos >= 0 && regions[insertPos].Score < key.Score {
			regions[insertPos+1] = regions[insertPos]
			insertPos--
		}

		regions[insertPos+1] = key
	}
}
