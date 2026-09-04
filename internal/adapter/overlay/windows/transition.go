//go:build windows

package windows

import (
	"context"
	"image"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/motion"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain"
)

// The recursive-grid depth transition on the Windows overlay window.
//
// A depth change zooms the cells from where the last depth left them to where
// the new depth puts them, on the curve the Linux Cairo path and CoreAnimation
// use (render/motion). A goroutine paces the frames; each one takes the
// manager's renderMu the way the mouse-action indicator's does, queues the
// interpolated cells and hands them to the overlay UI thread through Flush, so
// the painting and presenting stay on that thread and a frame the thread has
// not reached yet coalesces with the next.

// transitionPlan is what one frame of a depth transition paints, resolved once
// when the transition starts so a frame reads nothing that a later draw could
// have replaced.
type transitionPlan struct {
	fromRects, toRects []image.Rectangle
	keyRunes           []rune
	nextKeyRunes       []rune
	nextDims           domain.GridDimensions
	style              recursivegridcomponent.Style
	pointer            recursivegridcomponent.VirtualPointerState
	duration           time.Duration
}

// startTransition begins painting plan over its duration. The caller holds
// renderMu, and has canceled the transition before it.
func (o *winOverlay) startTransition(plan transitionPlan) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	o.transitionCancel = cancel
	o.transitionDone = done

	go o.runTransition(ctx, plan, done)
}

func (o *winOverlay) runTransition(ctx context.Context, plan transitionPlan, done chan struct{}) {
	defer close(done)

	startTime := time.Now()

	for {
		rawProgress := min(float64(time.Since(startTime))/float64(plan.duration), 1)

		frameStart := time.Now()

		o.renderMu.Lock()

		// The draw that canceled this transition may have been waiting for
		// the lock this frame now holds; its cancel is the last word.
		select {
		case <-ctx.Done():
			o.renderMu.Unlock()

			return
		default:
		}

		if o.window != nil && o.window.Healthy() {
			o.animRects = motion.LerpRects(
				plan.fromRects, plan.toRects, motion.EaseInOut(rawProgress),
			)
			o.paintRecursiveGrid(
				o.animRects,
				plan.keyRunes,
				plan.nextKeyRunes,
				plan.nextDims,
				plan.style,
				plan.pointer,
			)
		}

		if rawProgress >= 1 {
			o.transitionCancel = nil
			o.transitionDone = nil
		}

		o.renderMu.Unlock()

		if rawProgress >= 1 {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(motion.FrameInterval - time.Since(frameStart)):
		}
	}
}

// cancelTransition stops a running transition without waiting for its
// goroutine: the goroutine re-checks the cancel under renderMu before it
// paints, so a caller holding the lock has the last frame it will see on
// screen. Every draw that repaints this surface calls it first.
func (o *winOverlay) cancelTransition() {
	if o == nil || o.transitionCancel == nil {
		return
	}

	o.transitionCancel()
	o.transitionCancel = nil
	o.transitionDone = nil
}

// forgetTransition cancels a running transition and forgets the depth the
// surface last drew, so the next recursive-grid frame draws in place rather
// than zooming out of bounds that are gone: a cleared surface, a resized
// window and every other mode's draw all end here.
func (o *winOverlay) forgetTransition() {
	if o == nil {
		return
	}

	o.cancelTransition()
	o.hasLast = false
	o.animRects = nil
}
