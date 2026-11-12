//go:build linux || windows

package vision

import (
	"bytes"
	"context"
	"image"
	"math"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/ports"
)

// readFrame is a capture as these platforms take it, an RGBA buffer the
// garbage collector owns.
type readFrame = *image.RGBA

func retainFrame(frame readFrame) readFrame { return frame }

func releaseFrame(readFrame) {}

// sameFrame reports two captures of the same pixels.
func sameFrame(a, b readFrame) bool {
	return a.Rect == b.Rect && bytes.Equal(a.Pix, b.Pix)
}

// probeBandDivisor is how many bands of the region's height the probe band is,
// and probeBandMin its least height. probeInsetDivisor is the share of the
// region's width the band leaves out at each side, and probeInsetMin the
// least it leaves. The band is captured instead of the whole region while
// watching it settle, since each capture copies its pixels.
const (
	probeBandDivisor  = 8
	probeBandMin      = 32
	probeInsetDivisor = 20
	probeInsetMin     = 24
	// bandHalves halves a size to center the band.
	bandHalves = 2
)

// probeBand is the band through the middle of region that the watch
// captures. Scrolled content moves through it, so it changes when the region
// does. It stops short of both sides, where a scroll bar fades out for a
// second after a scroll and would keep the band changing.
func probeBand(region image.Rectangle) image.Rectangle {
	height := min(max(region.Dy()/probeBandDivisor, probeBandMin), region.Dy())
	top := region.Min.Y + (region.Dy()-height)/bandHalves

	inset := max(region.Dx()/probeInsetDivisor, probeInsetMin)
	if region.Dx() <= bandHalves*inset {
		inset = 0
	}

	return image.Rect(region.Min.X+inset, top, region.Max.X-inset, top+height)
}

// bandOf copies the pixels of frame, a capture of region, that band covers,
// for comparing with a capture of band alone. A frame at another scale than
// the region, as on a HiDPI display, maps band by that scale. A band the
// probe captures at a slightly different size compares as changed, which
// only reads the region sooner.
func bandOf(frame readFrame, region, band image.Rectangle) readFrame {
	scaleX := float64(frame.Rect.Dx()) / float64(region.Dx())
	scaleY := float64(frame.Rect.Dy()) / float64(region.Dy())

	pixels := image.Rect(
		frame.Rect.Min.X+int(float64(band.Min.X-region.Min.X)*scaleX),
		frame.Rect.Min.Y+int(float64(band.Min.Y-region.Min.Y)*scaleY),
		frame.Rect.Min.X+int(float64(band.Max.X-region.Min.X)*scaleX),
		frame.Rect.Min.Y+int(float64(band.Max.Y-region.Min.Y)*scaleY),
	).Intersect(frame.Rect)

	copied := image.NewRGBA(image.Rect(0, 0, pixels.Dx(), pixels.Dy()))
	for row := range pixels.Dy() {
		from := frame.PixOffset(pixels.Min.X, pixels.Min.Y+row)
		copy(
			copied.Pix[row*copied.Stride:(row+1)*copied.Stride],
			frame.Pix[from:from+copied.Stride],
		)
	}

	return copied
}

// maskEdge is how far past each rectangle maskOut clears, for the
// antialiased edge of what the overlay drew.
const maskEdge = 1

// maskOut zeroes the pixels of frame, a capture of shown, that drawnOn covers,
// in global pixels. A frame at another scale than shown maps them by that
// scale.
func maskOut(frame readFrame, shown image.Rectangle, drawnOn []image.Rectangle) {
	if shown.Empty() {
		return
	}

	scaleX := float64(frame.Rect.Dx()) / float64(shown.Dx())
	scaleY := float64(frame.Rect.Dy()) / float64(shown.Dy())

	for _, rect := range drawnOn {
		rect = rect.Inset(-maskEdge).Intersect(shown)
		if rect.Empty() {
			continue
		}

		pixels := image.Rect(
			frame.Rect.Min.X+int(math.Floor(float64(rect.Min.X-shown.Min.X)*scaleX)),
			frame.Rect.Min.Y+int(math.Floor(float64(rect.Min.Y-shown.Min.Y)*scaleY)),
			frame.Rect.Min.X+int(math.Ceil(float64(rect.Max.X-shown.Min.X)*scaleX)),
			frame.Rect.Min.Y+int(math.Ceil(float64(rect.Max.Y-shown.Min.Y)*scaleY)),
		).Intersect(frame.Rect)

		for y := pixels.Min.Y; y < pixels.Max.Y; y++ {
			clear(frame.Pix[frame.PixOffset(pixels.Min.X, y):frame.PixOffset(pixels.Max.X, y)])
		}
	}
}

// DetectSegmentsAsRead runs the segment detector on the frame the last fast
// read of region captured, so it captures nothing, which on Linux is a
// capture that takes the overlay off the screen. With no read of region it
// captures, as DetectSegments does.
func (a *Adapter) DetectSegmentsAsRead(
	ctx context.Context,
	region image.Rectangle,
	cfg config.HintsSegmentConfig,
) ([]*element.Element, error) {
	frame, _, remembered := a.lastRead.recall(region)
	if !remembered {
		return a.DetectSegments(ctx, region, cfg)
	}

	_, err := detectionRegion(ctx, region, "segment")
	if err != nil {
		return nil, err
	}

	return detectSegmentsIn(ctx, detectionFrameOf(frame, region), cfg)
}

// ReadLinesOnceChanged waits for region to change from what the last fast
// read of it captured and to settle, then reads it fast. One that never
// changes reads the last read's lines again. The watch captures only a band
// of region, and region is captured whole once, to compare and read.
func (a *Adapter) ReadLinesOnceChanged(
	ctx context.Context,
	region image.Rectangle,
) ([][]ports.TextPiece, error) {
	if region.Empty() {
		return a.ReadLines(ctx, region, ports.TextFast)
	}

	started := time.Now()
	band := probeBand(region)

	before, lines, remembered := a.lastRead.recall(region)

	var beforeBand readFrame
	if remembered {
		beforeBand = bandOf(before, region, band)
	}

	// The probes leave the overlay up, so what it drew is left out of each
	// comparison, and before, captured with it off, compares the same way.
	same := func(x, y readFrame) bool {
		drawnOn := a.overlayDrawnOn()
		maskOut(x, band, drawnOn)
		maskOut(y, band, drawnOn)

		return sameFrame(x, y)
	}

	_, unchanged, err := captureOnceChanged(ctx, beforeBand, remembered, settleFrames[readFrame]{
		capture: func() (readFrame, error) { return a.captureProbe(ctx, band) },
		same:    same,
		release: releaseFrame,
	})
	if err != nil {
		return nil, err
	}

	// The band is only the middle of region, so a scroll elsewhere in it, as
	// in a short panel, leaves the band unchanged. The whole region is
	// compared once before the last read's lines are kept.
	frame, captureErr := a.captureRegion(ctx, region)
	if captureErr != nil {
		return nil, captureErr
	}

	if unchanged = unchanged && sameFrame(frame, before); !unchanged {
		lines, err = a.readFrameLines(ctx, region, frame, ports.TextFast)
		if err != nil {
			return nil, err
		}

		a.lastRead.remember(region, frame, lines)
	}

	a.logger.Debug("Read lines once changed",
		zap.Bool("unchanged", unchanged),
		zap.Int("line_count", len(lines)),
		zap.Duration("duration", time.Since(started)))

	return lines, nil
}
