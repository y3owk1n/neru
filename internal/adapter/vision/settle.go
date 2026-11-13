package vision

import (
	"context"
	"image"
	"sync"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	// settleInterval is the time between the captures that watch a region
	// settle.
	settleInterval = 40 * time.Millisecond
	// settleLimit bounds the watch. A scroll at the end of a page changes
	// nothing, and an animation never stops, so the watch ends here and the
	// region is read as it is.
	settleLimit = 500 * time.Millisecond
)

// settleFrames is how one platform captures a region, compares two captures
// and gives back one it no longer needs.
type settleFrames[F any] struct {
	capture func() (F, error)
	same    func(a, b F) bool
	release func(F)
}

// captureOnceChanged captures a region until it has changed from before, the
// frame the last read of it captured, and then stays the same for one
// settleInterval, and returns that capture. A capture equal to before is
// still the screen before the app moved, so it never counts as settled. With
// no before, the first change is taken as already seen. When the watch runs
// out at settleLimit it returns the last capture, and reports unchanged when
// that is still before.
func captureOnceChanged[F any](
	ctx context.Context,
	before F,
	hasBefore bool,
	frames settleFrames[F],
) (F, bool, error) {
	var zero F

	deadline := time.Now().Add(settleLimit)

	last, err := frames.capture()
	if err != nil {
		return zero, false, err
	}

	changed := !hasBefore || !frames.same(last, before)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			frames.release(last)

			return zero, false, derrors.Wrap(
				ctx.Err(),
				derrors.CodeContextCanceled,
				"operation canceled",
			)
		case <-time.After(settleInterval):
		}

		next, err := frames.capture()
		if err != nil {
			frames.release(last)

			return zero, false, err
		}

		settled := changed && frames.same(next, last)
		changed = changed || !frames.same(next, before)

		frames.release(last)
		last = next

		if settled {
			return last, false, nil
		}
	}

	return last, !changed, nil
}

// lastRead is the frame the last fast read of a region captured, with the
// lines read from it, so a read once the region changes knows what it is
// waiting to change from, and a region that never changed reads the same
// lines again without recognizing them.
type lastRead struct {
	mu     sync.Mutex
	region image.Rectangle
	frame  readFrame
	lines  [][]ports.TextPiece
	ok     bool
}

// recall returns what the last read of region left, if it was of region.
func (r *lastRead) recall(region image.Rectangle) (readFrame, [][]ports.TextPiece, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.ok || r.region != region {
		var zero readFrame

		return zero, nil, false
	}

	return retainFrame(r.frame), r.lines, true
}

// forget gives back the frame it held, so nothing is remembered.
func (r *lastRead) forget() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ok {
		releaseFrame(r.frame)
	}

	var zero readFrame

	r.region, r.frame, r.lines, r.ok = image.Rectangle{}, zero, nil, false
}

// remember keeps frame and lines as the last read of region, and gives back
// the frame it held.
func (r *lastRead) remember(region image.Rectangle, frame readFrame, lines [][]ports.TextPiece) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ok {
		releaseFrame(r.frame)
	}

	r.region, r.frame, r.lines, r.ok = region, retainFrame(frame), lines, true
}
