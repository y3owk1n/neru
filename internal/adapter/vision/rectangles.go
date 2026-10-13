package vision

import (
	"cmp"
	"context"
	"image"
	"slices"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/vision/contour"
	"github.com/y3owk1n/neru/internal/config"
)

// rectangleParams are the contour detector's shipped thresholds. The rectangle
// half of hints.vision has only the four rectangle_* filters of its own, and
// the hints.contour section tunes the contour strategy, not this.
var rectangleParams = contour.Params{
	EdgeLowThreshold:  config.DefaultContourEdgeLowThreshold,
	EdgeHighThreshold: config.DefaultContourEdgeHighThreshold,
	MinTargetWidth:    config.DefaultContourMinTargetWidth,
	MinTargetHeight:   config.DefaultContourMinTargetHeight,
	MaxTargetWidth:    config.DefaultContourMaxTargetWidth,
	MaxTargetHeight:   config.DefaultContourMaxTargetHeight,
	FlatLineHeight:    config.DefaultContourFlatLineHeight,
	ContainerHeight:   config.DefaultContourContainerHeight,
	SameCenterSlack:   config.DefaultContourSameCenterSlack,
	SquareIconSize:    config.DefaultContourSquareIconSize,
	SquareIconSlack:   config.DefaultContourSquareIconSlack,
}

// rectangleRegions finds button shapes in frame, captured from region, with
// the contour detector, and returns the ones the rectangle_* filters keep, in
// global coordinates. macOS answers the same options with Vision's rectangle
// request.
//
// rectangle_min_size is a fraction of the region's shorter side, which a
// box's shorter side must reach. The aspect bounds are width over height.
// rectangle_max_candidates keeps the largest boxes, since the detector scores
// none. Every box it returns scores one for the same reason.
func rectangleRegions(
	ctx context.Context,
	frame *image.RGBA,
	region image.Rectangle,
	cfg config.HintsVisionConfig,
) ([]DetectedRegion, error) {
	scale := float64(frame.Rect.Dy()) / float64(region.Dy())

	rects, err := contour.Detect(ctx, frame, scale, rectangleParams)
	if err != nil {
		return nil, err
	}

	minSide := cfg.RectangleMinSize * float64(min(region.Dx(), region.Dy()))
	regions := make([]DetectedRegion, 0, len(rects))

	for _, rect := range rects {
		bounds := rect.Add(region.Min).Intersect(region)
		if bounds.Empty() {
			continue
		}

		width, height := float64(bounds.Dx()), float64(bounds.Dy())
		aspect := width / height

		if min(width, height) < minSide ||
			aspect < cfg.RectangleMinAspect || aspect > cfg.RectangleMaxAspect {
			continue
		}

		regions = append(regions, DetectedRegion{Bounds: bounds, Score: 1})
	}

	slices.SortStableFunc(regions, func(a, b DetectedRegion) int {
		return cmp.Compare(area(b.Bounds), area(a.Bounds))
	})

	if len(regions) > cfg.RectangleMaxCandidates {
		regions = regions[:cfg.RectangleMaxCandidates]
	}

	return regions, nil
}

// withoutTextOverlap drops the rectangles that a text region overlaps by at
// least iouThreshold. The text region already hints that target and carries
// its label for hint search, which a rectangle would take away if it won the
// merge.
func withoutTextOverlap(
	rects []DetectedRegion,
	text []DetectedRegion,
	iouThreshold float64,
) []DetectedRegion {
	if iouThreshold <= 0 {
		iouThreshold = config.DefaultVisionMergeIOUThreshold
	}

	kept := rects[:0]

	for _, rect := range rects {
		covered := slices.ContainsFunc(text, func(word DetectedRegion) bool {
			return intersectionOverUnion(rect.Bounds, word.Bounds) >= iouThreshold
		})
		if !covered {
			kept = append(kept, rect)
		}
	}

	return kept
}

// startRectangles runs rectangleRegions in the background when
// hints.vision.detect_rectangles is on, bounded by request_timeout_ms, so it
// overlaps the text recognition. The returned function waits for the regions
// and the error, the timeout included. A caller with text results keeps them
// and drops the error, since the text half still answers.
func (a *Adapter) startRectangles(
	ctx context.Context,
	frame *image.RGBA,
	region image.Rectangle,
	cfg config.HintsVisionConfig,
) func() ([]DetectedRegion, error) {
	if !cfg.DetectRectangles {
		return func() ([]DetectedRegion, error) { return nil, nil }
	}

	type result struct {
		regions []DetectedRegion
		err     error
	}

	done := make(chan result, 1)

	go func() {
		ctx, cancel := context.WithTimeout(
			ctx,
			time.Duration(cfg.RequestTimeoutMS)*time.Millisecond,
		)
		defer cancel()

		started := time.Now()

		regions, err := rectangleRegions(ctx, frame, region, cfg)
		if err != nil {
			a.logger.Debug("Rectangle detection failed",
				zap.Duration("duration", time.Since(started)), zap.Error(err))
		} else {
			a.logger.Debug("Rectangle detection complete",
				zap.Duration("duration", time.Since(started)),
				zap.Int("rectangles", len(regions)))
		}

		done <- result{regions, err}
	}()

	return func() ([]DetectedRegion, error) {
		found := <-done

		return found.regions, found.err
	}
}

func area(rect image.Rectangle) int {
	return rect.Dx() * rect.Dy()
}
