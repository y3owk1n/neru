//go:build linux

package linux

import (
	"math"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// hyprlandScrollSession injects an animated modified scroll on Hyprland: the
// modifier held on the virtual keyboard for the length of the animation, and
// every chunk sent as whole uinput notches.
//
// It exists because the session the rest of Wayland uses cannot serve this
// case. That one carries its chunks on the virtual pointer, which is the half
// Hyprland drops once a virtual-keyboard modifier is down (#1474) — so an
// animated zoom would ease its way to nothing. Bypassing the animator instead
// would answer that by taking smooth_scroll away from modified scrolls without
// saying so, and a setting that silently stops applying to one binding is worse
// than one that costs a wheel notch of precision.
//
// Granularity is a whole notch rather than zero for the same reason X11's
// session reports one: uinput scrolling is REL_WHEEL clicks, and there is no
// sub-notch value to hand it. What the animation still does is spread those
// notches over the same eased curve every other backend uses.
type hyprlandScrollSession struct {
	pressed action.Modifiers
}

// newHyprlandScrollSession presses the modifiers and holds them for the whole
// animation, so twenty chunks read as one zoom rather than twenty.
func newHyprlandScrollSession(modifiers action.Modifiers) (scrollSession, error) {
	err := waylandScrollBackendAvailable()
	if err != nil {
		return nil, err
	}

	pressed, err := pressWaylandModifiers(modifiers)
	if err != nil {
		return nil, err
	}

	waitForWaylandModifierPress()

	return &hyprlandScrollSession{pressed: pressed}, nil
}

func (s *hyprlandScrollSession) granularity() float64 { return scrollPixelsPerNotch }

func (s *hyprlandScrollSession) inject(deltaX, deltaY float64) error {
	err := s.notchAxis(uinputScrollAxisVertical, deltaY)
	if err != nil {
		return err
	}

	return s.notchAxis(uinputScrollAxisHorizontal, deltaX)
}

// notchAxis emits one chunk as whole wheel notches. The animator only ever
// hands it exact multiples of a notch, so the rounding here settles
// floating-point error rather than a real fraction.
func (s *hyprlandScrollSession) notchAxis(axis int, delta float64) error {
	if delta == 0 {
		return nil
	}

	notches := int(math.Round(math.Abs(delta) / scrollPixelsPerNotch))
	if notches == 0 {
		return nil
	}

	value := 1
	if delta < 0 {
		value = -1
	}

	batch := make([]int, notches)
	for i := range batch {
		batch[i] = value
	}

	return uinputScrollBatch(axis, batch)
}

// close lets go of only what this session pressed, so a modifier the user is
// physically holding survives the animation.
func (s *hyprlandScrollSession) close() {
	if s.pressed == 0 {
		return
	}

	waitForScrollDelivery()

	_ = releaseWaylandModifiers(s.pressed)
}
