package vision

import (
	"image"
	"math"

	"github.com/y3owk1n/neru/internal/config"
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
type regionClassifier struct {
	cfg config.HintsVisionConfig
}

const roleButton = "AXButton"

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
	case region.IsText && c.isLikelyButton(aspectRatio, region.Score, width, height):
		return roleButton, true
	case region.IsText && c.isLikelyLink(aspectRatio, width, height):
		return "AXLink", true
	case region.IsText:
		return "AXStaticText", false
	case c.isLikelyCheckBox(aspectRatio, width, height):
		return "AXCheckBox", true
	case c.isLikelyButton(aspectRatio, region.Score, width, height):
		return roleButton, true
	case c.isLikelyImage(aspectRatio, width, height):
		return "AXImage", false
	default:
		minConf := 0.5
		if c.cfg.GenericClickableMinConfidence > 0 {
			minConf = c.cfg.GenericClickableMinConfidence
		}

		if region.Score > minConf {
			return roleButton, true
		}

		return "AXGenericElement", false
	}
}

// isLikelyButton checks aspect ratio and saliency to guess "button".
// Buttons and toolbar icons are typically 0.8:1 to 8:1 width:height
// (square-ish to wide) with decent saliency. For near-square regions,
// size is constrained to avoid catching large images.
func (c *regionClassifier) isLikelyButton(aspectRatio, score, width, height float64) bool {
	minAspect := 0.8
	if c.cfg.ButtonMinAspect > 0 {
		minAspect = c.cfg.ButtonMinAspect
	}

	maxAspect := 8.0
	if c.cfg.ButtonMaxAspect > 0 {
		maxAspect = c.cfg.ButtonMaxAspect
	}

	minConf := 0.3
	if c.cfg.ButtonMinConfidence > 0 {
		minConf = c.cfg.ButtonMinConfidence
	}

	maxIconSize := 48
	if c.cfg.ButtonIconMaxSize > 0 {
		maxIconSize = c.cfg.ButtonIconMaxSize
	}

	if aspectRatio < minAspect || aspectRatio > maxAspect || score < minConf {
		return false
	}

	// Near-square regions: limit size to avoid classifying images as buttons
	if aspectRatio >= minAspect && aspectRatio <= 1.5 {
		return max(width, height) <= float64(maxIconSize)
	}

	return true
}

// isLikelyLink checks for long, narrow text (links).
func (c *regionClassifier) isLikelyLink(aspectRatio, width, height float64) bool {
	minAspect := 5.0
	if c.cfg.LinkMinAspect > 0 {
		minAspect = c.cfg.LinkMinAspect
	}

	maxHeight := 40
	if c.cfg.LinkMaxHeight > 0 {
		maxHeight = c.cfg.LinkMaxHeight
	}

	minWidth := 50
	if c.cfg.LinkMinWidth > 0 {
		minWidth = c.cfg.LinkMinWidth
	}

	return aspectRatio > minAspect && height < float64(maxHeight) && width > float64(minWidth)
}

// isLikelyImage checks for near-square regions that are large enough to be
// actual images rather than buttons or icons. Images >= 48px in both dimensions
// with near-square aspect are classified as non-interactive.
func (c *regionClassifier) isLikelyImage(aspectRatio, width, height float64) bool {
	minSize := 48
	if c.cfg.ImageMinSize > 0 {
		minSize = c.cfg.ImageMinSize
	}

	return aspectRatio >= 0.8 && aspectRatio <= 1.5 && width >= float64(minSize) &&
		height >= float64(minSize)
}

// isLikelyCheckBox checks for small square-ish regions (< 32px in both dims).
func (c *regionClassifier) isLikelyCheckBox(aspectRatio, width, height float64) bool {
	maxSize := 32
	if c.cfg.CheckboxMaxSize > 0 {
		maxSize = c.cfg.CheckboxMaxSize
	}

	return aspectRatio >= 0.8 && aspectRatio <= 1.5 && width < float64(maxSize) &&
		height < float64(maxSize)
}

// MergeRegions merges overlapping regions using non-maximum suppression.
// Regions with higher scores suppress lower-scoring overlaps (IoU > iouThreshold).
func MergeRegions(regions []DetectedRegion, iouThreshold float64) []DetectedRegion {
	if len(regions) == 0 {
		return nil
	}

	if iouThreshold <= 0 {
		iouThreshold = 0.5
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
			if iou < iouThreshold {
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
