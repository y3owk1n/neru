//go:build windows

package windows

import (
	"math"

	"github.com/y3owk1n/neru/internal/adapter/platform/scrollanim"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// scrollGranularity is the smallest distance a wheel event can carry, in the
// caller's pixels. MOUSEINPUT.mouseData is an integer count of WHEEL_DELTA/120ths
// and wheelUnitsPerPixel of them make a pixel, so a step can be as short as a
// quarter pixel, one 120th of a notch. That is what makes this animate below a
// notch the way Wayland does and X11 cannot.
const scrollGranularity = 1.0 / wheelUnitsPerPixel

// sendInputScrollSession injects wheel events through SendInput with a
// modifier hold open for the length of the animation.
type sendInputScrollSession struct {
	hold modifierHold
}

func newSendInputScrollSession(modifiers action.Modifiers) (scrollanim.Session, error) {
	hold, err := holdModifiers(modifiers)
	if err != nil {
		return nil, err
	}

	return &sendInputScrollSession{hold: hold}, nil
}

// Granularity is one wheel unit, so every chunk converts to mouseData without
// rounding.
func (s *sendInputScrollSession) Granularity() float64 { return scrollGranularity }

// Inject sends one chunk as wheel records.
func (s *sendInputScrollSession) Inject(deltaX, deltaY float64) error {
	// This is the sign convention wheelEvents applies. Positive deltaY is up and
	// positive deltaX is left, and only the horizontal axis needs negating.
	records := wheelRecords(
		int32(math.Round(deltaY*wheelUnitsPerPixel)),
		int32(math.Round(-deltaX*wheelUnitsPerPixel)),
	)

	for _, event := range records {
		err := sendMouseInput(event.flags, event.data)
		if err != nil {
			return err
		}
	}

	return nil
}

// Close releases the modifier hold.
func (s *sendInputScrollSession) Close() {
	s.hold.release()
}

// scrollAnim spreads every animated scroll over time. Wheel units are a
// quarter pixel, fine enough that no request needs a cap.
var scrollAnim = scrollanim.New(newSendInputScrollSession, 0)
