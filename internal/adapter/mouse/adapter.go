package mouse

import (
	"context"
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/ports"
)

// backend is one platform's mouse injection. Each backend_<os>.go builds one.
type backend struct {
	isDown     func(button action.MouseButton) bool
	releaseAll func()
	click      func(point image.Point, button action.MouseButton, modifiers action.Modifiers) error
	down       func(point image.Point, button action.MouseButton, modifiers action.Modifiers) error
	up         func(point image.Point, button action.MouseButton, modifiers action.Modifiers) error
	move       func(point image.Point) error
	scroll     func(deltaX, deltaY int, modifiers action.Modifiers) error
}

// Adapter implements ports.MousePort.
type Adapter struct {
	logger  *zap.Logger
	backend backend
}

// NewAdapter creates the mouse adapter for this platform. A nil logger is
// replaced with a no-op.
func NewAdapter(logger *zap.Logger) *Adapter {
	return newAdapter(logger, platformBackend())
}

func newAdapter(logger *zap.Logger, b backend) *Adapter {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Adapter{logger: logger.Named("mouse"), backend: b}
}

// PerformActionAtPoint executes a mouse action at the specified point.
func (a *Adapter) PerformActionAtPoint(
	ctx context.Context,
	actionType action.Type,
	point image.Point,
	modifiers action.Modifiers,
) error {
	err := checkContext(ctx)
	if err != nil {
		return err
	}

	if ce := a.logger.Check(zap.DebugLevel, "Performing action at point"); ce != nil {
		ce.Write(
			zap.String("action", actionType.String()),
			zap.Int("x", point.X),
			zap.Int("y", point.Y),
			zap.String("modifiers", modifiers.String()),
		)
	}

	err = a.perform(actionType, point, modifiers)
	if err != nil {
		return derrors.Wrap(err, derrors.CodeActionFailed, "failed to perform action at point")
	}

	return nil
}

// Scroll performs a scroll action at the current cursor position.
func (a *Adapter) Scroll(
	_ context.Context,
	deltaX, deltaY int,
	modifiers action.Modifiers,
) error {
	if ce := a.logger.Check(zap.DebugLevel, "Performing scroll"); ce != nil {
		ce.Write(
			zap.Int("delta_x", deltaX),
			zap.Int("delta_y", deltaY),
			zap.String("modifiers", modifiers.String()),
		)
	}

	a.backend.releaseAll()

	err := a.backend.scroll(deltaX, deltaY, modifiers)
	if err != nil {
		// A backend refusing a modifier it cannot present says so with
		// CodeNotSupported, and derrors matches only the outermost code, so
		// wrapping here would turn "this platform cannot do that" into a
		// generic failure before any caller could tell them apart.
		if derrors.IsNotSupported(err) {
			return err
		}

		return derrors.Wrap(err, derrors.CodeActionFailed, "failed to scroll")
	}

	return nil
}

// ReleaseHeldButtons releases any mouse button this process still holds down.
func (a *Adapter) ReleaseHeldButtons(ctx context.Context) error {
	err := checkContext(ctx)
	if err != nil {
		return err
	}

	a.backend.releaseAll()

	return nil
}

func (a *Adapter) perform(
	actionType action.Type,
	point image.Point,
	modifiers action.Modifiers,
) error {
	var err error

	switch actionType {
	case action.TypeLeftClick:
		// A left click while the left button is held ends that drag at the
		// click point, as left_click --state up would. A full click there would
		// press over the held button and leave it recorded as held.
		if a.backend.isDown(action.ButtonLeft) {
			err = a.backend.up(point, action.ButtonLeft, modifiers)

			a.backend.releaseAll()

			break
		}

		a.backend.releaseAll()

		err = a.backend.click(point, action.ButtonLeft, modifiers)
	case action.TypeRightClick:
		a.backend.releaseAll()

		err = a.backend.click(point, action.ButtonRight, modifiers)
	case action.TypeMiddleClick:
		a.backend.releaseAll()

		err = a.backend.click(point, action.ButtonMiddle, modifiers)
	case action.TypeLeftMouseDown, action.TypeLeftMouseUp,
		action.TypeRightMouseDown, action.TypeRightMouseUp,
		action.TypeMiddleMouseDown, action.TypeMiddleMouseUp,
		action.TypeLeftMouseToggle, action.TypeRightMouseToggle, action.TypeMiddleMouseToggle:
		err = a.pressOrRelease(actionType, point, modifiers)
	case action.TypeMoveMouse, action.TypeMoveMouseRelative:
		err = a.backend.move(point)
	case action.TypeScroll:
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"scroll actions should use Scroll method: %s",
			actionType,
		)
	default:
		return derrors.Newf(derrors.CodeInvalidInput, "unsupported action type: %s", actionType)
	}

	if err != nil {
		return derrors.Wrap(err, derrors.CodeActionFailed, "failed to perform action")
	}

	return nil
}

// pressOrRelease presses or releases the button addressed by actionType,
// resolving a toggle against the buttons this process holds.
func (a *Adapter) pressOrRelease(
	actionType action.Type,
	point image.Point,
	modifiers action.Modifiers,
) error {
	button, phase, ok := actionType.MouseButtonPhase()
	if !ok {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"not a mouse button action: %s",
			actionType,
		)
	}

	if phase == action.PhaseToggle {
		if a.backend.isDown(button) {
			phase = action.PhaseUp
		} else {
			phase = action.PhaseDown
		}
	}

	if phase == action.PhaseUp {
		return a.backend.up(point, button, modifiers)
	}

	return a.backend.down(point, button, modifiers)
}

func checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
		return nil
	}
}

var _ ports.MousePort = (*Adapter)(nil)
