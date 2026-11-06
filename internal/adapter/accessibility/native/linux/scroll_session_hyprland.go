//go:build linux

package linux

import (
	"math"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// hyprlandScrollSession injects an animated modified scroll on Hyprland, the
// modifier held on the virtual keyboard for the animation and every chunk sent
// as whole uinput notches. The usual Wayland session uses the virtual pointer,
// which Hyprland drops once a virtual-keyboard modifier is down (#1474).
// Bypassing the animator would silently take smooth_scroll from modified
// scrolls, which is worse than losing sub-notch precision. Granularity is a
// whole notch, as on X11, because uinput scrolls in REL_WHEEL clicks.
type hyprlandScrollSession struct {
	pressed action.Modifiers
	restore func()
}

// newHyprlandScrollSession presses the modifiers and holds them for the whole
// animation, so twenty chunks read as one zoom rather than twenty.
func newHyprlandScrollSession(modifiers action.Modifiers) (scrollSession, error) {
	err := waylandScrollBackendAvailable()
	if err != nil {
		return nil, err
	}

	restore := liftPhysicalModifiers()

	pressed, err := pressWaylandModifiers(modifiers)
	if err != nil {
		restore()

		return nil, err
	}

	waitForWaylandModifierPress()

	return &hyprlandScrollSession{pressed: pressed, restore: restore}, nil
}

func (s *hyprlandScrollSession) granularity() float64 { return scrollPixelsPerNotch }

func (s *hyprlandScrollSession) inject(deltaX, deltaY float64) error {
	err := s.notchAxis(uinputScrollAxisVertical, deltaY)
	if err != nil {
		return err
	}

	return s.notchAxis(uinputScrollAxisHorizontal, deltaX)
}

// notchAxis emits one chunk as whole wheel notches. The animator hands it
// exact notch multiples, so rounding only settles floating-point error.
func (s *hyprlandScrollSession) notchAxis(axis int, delta float64) error {
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
	defer s.restore()

	if s.pressed == 0 {
		return
	}

	waitForScrollDelivery()

	_ = releaseWaylandModifiers(s.pressed)
}
