//go:build linux || windows

package vision

import (
	"bytes"
	"context"
	"image"
	"time"

	"go.uber.org/zap"

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

// ReadLinesOnceChanged waits for region to change from what the last fast
// read of it captured and to settle, then reads it fast. One that never
// changes reads the last read's lines again. The watch captures only a band
// of region, and region is captured whole once, to read.
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

	_, unchanged, err := captureOnceChanged(ctx, beforeBand, remembered, settleFrames[readFrame]{
		capture: func() (readFrame, error) { return a.captureRegion(ctx, band) },
		same:    sameFrame,
		release: releaseFrame,
	})
	if err != nil {
		return nil, err
	}

	if !unchanged {
		frame, captureErr := a.captureRegion(ctx, region)
		if captureErr != nil {
			return nil, captureErr
		}

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
