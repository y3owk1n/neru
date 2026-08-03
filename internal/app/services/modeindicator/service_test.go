package modeindicator_test

import (
	"context"
	"errors"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services/modeindicator"
	"github.com/y3owk1n/neru/internal/derrors"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// TestService_GetCursorPositionWithoutSystemPort pins the degraded path. The
// service is constructed during startup and can outlive a failed system-port
// init, so a nil port must produce an error rather than a panic.
func TestService_GetCursorPositionWithoutSystemPort(t *testing.T) {
	t.Parallel()

	service := modeindicator.NewService(nil, nil, nil)

	_, _, err := service.GetCursorPosition(t.Context())
	if err == nil {
		t.Fatal("GetCursorPosition() error = nil, want an error for a nil system port")
	}

	if !derrors.IsCode(err, derrors.CodeActionFailed) {
		t.Errorf("GetCursorPosition() code = %v, want CodeActionFailed", err)
	}
}

func TestService_GetCursorPositionReturnsPortCoordinates(t *testing.T) {
	t.Parallel()

	system := &portmocks.MockSystemPort{
		CursorPositionFunc: func(context.Context) (image.Point, error) {
			return image.Point{X: 42, Y: 99}, nil
		},
	}

	service := modeindicator.NewService(system, nil, nil)

	posX, posY, err := service.GetCursorPosition(t.Context())
	if err != nil {
		t.Fatalf("GetCursorPosition() error = %v, want nil", err)
	}

	if posX != 42 || posY != 99 {
		t.Errorf("GetCursorPosition() = (%d,%d), want (42,99)", posX, posY)
	}
}

// errCursorUnavailable stands in for a backend failure.
var errCursorUnavailable = errors.New("cursor unavailable")

// TestService_GetCursorPositionWrapsPortFailure pins that a backend error is
// wrapped rather than returned raw, so callers see a domain error code.
func TestService_GetCursorPositionWrapsPortFailure(t *testing.T) {
	t.Parallel()

	system := &portmocks.MockSystemPort{
		CursorPositionFunc: func(context.Context) (image.Point, error) {
			return image.Point{}, errCursorUnavailable
		},
	}

	service := modeindicator.NewService(system, nil, nil)

	_, _, err := service.GetCursorPosition(t.Context())
	if err == nil {
		t.Fatal("GetCursorPosition() error = nil, want the wrapped port failure")
	}

	if !errors.Is(err, errCursorUnavailable) {
		t.Errorf("GetCursorPosition() error = %v, want it to wrap %v", err, errCursorUnavailable)
	}
}

// TestService_UpdateIndicatorPositionForwardsToOverlay pins the draw call, the
// service's only side effect.
func TestService_UpdateIndicatorPositionForwardsToOverlay(t *testing.T) {
	t.Parallel()

	overlay := &portmocks.MockOverlayPort{}
	service := modeindicator.NewService(nil, overlay, nil)

	service.UpdateIndicatorPosition(7, 11)

	gotX, gotY := overlay.LastModeIndicatorPosition()
	if gotX != 7 || gotY != 11 {
		t.Errorf("DrawModeIndicator got (%d,%d), want (7,11)", gotX, gotY)
	}
}
