package modes

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/event"
)

// snapScope is what a snap searches from and within, read in one hold of
// h.mu, with the token that tells whether the user moved on while it scanned.
type snapScope struct {
	// region is the area the mode narrowed the screen to, valid when
	// narrowed is set.
	region   image.Rectangle
	narrowed bool
	// selection is the mode's selection, valid when fromSelection is set.
	// A snap inside a narrowed region searches from it, and from the cursor
	// when the mode holds none.
	selection     image.Point
	fromSelection bool
	// overlayDrawn says a mode has something on screen a capture could see.
	overlayDrawn bool
	bare         bool
	// mode names the active mode, as the select event names it.
	mode string

	// session, writes and holding are what movedOnSince compares.
	session uint64
	writes  uint64
	holding bool
}

// SnapOptions is what `neru action snap` was asked for.
type SnapOptions struct {
	// Strategy overrides the strategy hints would use for the focused app.
	Strategy string
	// Roles and Text narrow the elements as hints --role and --text do.
	Roles []string
	Text  []string
	// Direction, when Directed is set, steps to the next element that way
	// instead of the nearest one.
	Direction domain.Direction
	Directed  bool
	// Bare ignores the mode's region and searches from the cursor.
	Bare bool
}

// SnapAction is the action a snap names on the select event it publishes.
const SnapAction = "snap"

// Snap moves the cursor onto the nearest element hints would label, or with a
// direction onto the next one that way, and returns where it landed.
//
// Inside a region the active mode has narrowed the screen to, it searches
// that region from the mode's selection, or from the cursor when the mode
// holds none, such as after a move_mouse_relative. Anywhere else it searches
// the active screen from the cursor. Bare ignores the mode's region and searches from the cursor.
//
// Then the cursor moves onto the element, the mode's selection moves with it
// so a later click aims at that exact point, and the mode stays open. A select
// event naming snap reports it.
//
// The scan runs off h.mu, so keys typed meanwhile still reach the mode. A
// snap whose mode changed, or whose region or selection moved, during the
// scan is dropped rather than pulling the cursor somewhere the user has
// already left. The selection moves in the same hold as that check, so a
// cursor move that then fails still leaves a later click aimed at the element.
//
// Snaps run one at a time. Inside a mode, a held key's first press and its
// first repeat can overlap, and two directed snaps from the same cursor would
// land on the same element and lose a step.
func (h *Handler) Snap(ctx context.Context, opts SnapOptions) (element.Target, error) {
	h.snapMu.Lock()
	defer h.snapMu.Unlock()

	scope := h.readSnapScope(opts.Bare)

	origin := scope.selection
	if !scope.fromSelection {
		cursor, err := h.actionService.CursorPositionForAction(ctx)
		if err != nil {
			return element.Target{}, err
		}

		origin = cursor
	}

	region, captureRegion := scope.region, scope.region
	if !scope.narrowed {
		screen, err := h.actionService.ScreenBounds(ctx)
		if err != nil {
			return element.Target{}, err
		}

		region, captureRegion = screen, image.Rectangle{}
	}

	scanCtx, cancel := context.WithTimeout(ctx, HintTimeout)
	defer cancel()

	elements, err := h.hintService.SnapCandidates(scanCtx, services.SnapScan{
		Roles:        opts.Roles,
		Text:         opts.Text,
		Strategy:     opts.Strategy,
		Region:       captureRegion,
		OverlayDrawn: scope.overlayDrawn,
	})
	if err != nil {
		return element.Target{}, h.unlessMovedOn(scope, err)
	}

	target, found := element.Nearest(elements, origin, region)
	if opts.Directed {
		target, found = element.Toward(elements, origin, region, opts.Direction)
	}

	if !found {
		message := "no element to snap to"
		if opts.Directed {
			message = "no element to snap to " + opts.Direction.String() + " of the cursor"
		}

		return element.Target{}, h.unlessMovedOn(
			scope, derrors.New(derrors.CodeElementNotFound, message),
		)
	}

	if !h.placeSelectionForSnap(scope, target.Point) {
		return element.Target{}, errMovedOn
	}

	err = h.actionService.MoveCursorToPointAndWait(ctx, target.Point)
	if err != nil {
		return element.Target{}, err
	}

	h.events.Publish(event.Event{
		Name:   event.Select,
		Mode:   scope.mode,
		Action: SnapAction,
		Point:  target.Point,
	})

	return target, nil
}

// readSnapScope reads what a snap needs from the active mode.
func (h *Handler) readSnapScope(bare bool) snapScope {
	h.mu.Lock()
	defer h.mu.Unlock()

	scope := snapScope{
		overlayDrawn: h.appState.CurrentMode() != domain.ModeIdle,
		mode:         h.CurrModeString(),
		bare:         bare,
		session:      h.modeSession,
	}

	tracker, tracks := activeModeExtension[selectionTracker](&h.handlerState)
	if tracks {
		scope.writes = tracker.SelectionWrites()
		_, scope.holding = tracker.SelectionPoint()
	}

	if bare {
		return scope
	}

	reporter, reports := activeModeExtension[selectionRegionReporter](&h.handlerState)
	if !reports {
		return scope
	}

	scope.region, scope.narrowed = reporter.SelectionRegion()
	if scope.narrowed && tracks {
		scope.selection, scope.fromSelection = tracker.SelectionPoint()
	}

	return scope
}

// errMovedOn is the answer of a snap whose mode, region or selection moved
// while it scanned.
var errMovedOn = derrors.New(derrors.CodeContextCanceled, "the mode moved on during the scan")

// unlessMovedOn returns err, or errMovedOn when the user moved on during the
// scan. A snap that failed is then dropped like one that succeeded too late,
// so the rest of its binding does not act on a target the user has left.
func (h *Handler) unlessMovedOn(scope snapScope, err error) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.movedOnSince(scope) {
		return errMovedOn
	}

	return err
}

// placeSelectionForSnap moves the active mode's selection onto point, where
// the snap is about to put the cursor. A click after the snap then aims at
// that exact point, as one after move_mouse does, instead of reading the
// cursor back from a platform that may not have caught up with the move yet.
// It reports false, and moves nothing, when the user moved on during the scan.
func (h *Handler) placeSelectionForSnap(scope snapScope, point image.Point) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.movedOnSince(scope) {
		return false
	}

	tracker, tracks := activeModeExtension[selectionTracker](&h.handlerState)
	if tracks {
		tracker.PlaceSelectionPoint(point)
	}

	return true
}

// movedOnSince reports whether the mode changed, or its region or selection
// moved, since scope was read. It compares the region on its own, because a
// reset or a backspace out of a grid subgrid clears the selection without
// writing one. It also compares whether a selection is held, because the
// start of a held glide clears it the same way. Caller must hold h.mu.
func (h *handlerState) movedOnSince(scope snapScope) bool {
	if h.modeSession != scope.session {
		return true
	}

	if !scope.bare {
		var region image.Rectangle

		narrowed := false

		reporter, reports := activeModeExtension[selectionRegionReporter](h)
		if reports {
			region, narrowed = reporter.SelectionRegion()
		}

		if narrowed != scope.narrowed || region != scope.region {
			return true
		}
	}

	tracker, tracks := activeModeExtension[selectionTracker](h)
	if !tracks {
		return false
	}

	_, holding := tracker.SelectionPoint()

	return tracker.SelectionWrites() != scope.writes || holding != scope.holding
}
