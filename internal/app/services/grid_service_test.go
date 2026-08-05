package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports/mocks"
)

func TestGridService_Health(t *testing.T) {
	overlayDown := derrors.New(derrors.CodeNotSupported, "no display server")

	tests := []struct {
		name       string
		overlayErr error
	}{
		{name: "healthy overlay", overlayErr: nil},
		{name: "unhealthy overlay", overlayErr: overlayDown},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockOverlay := &mocks.MockOverlayPort{
				HealthFunc: func(_ context.Context) error {
					return testCase.overlayErr
				},
			}

			checks := services.NewGridService(mockOverlay).Health(context.Background())

			if len(checks) != 1 {
				t.Fatalf("Health() reported %d checks, want 1 (overlay only)", len(checks))
			}

			got, ok := checks["overlay"]
			if !ok {
				t.Fatal(`Health() has no "overlay" check`)
			}

			if !errors.Is(got, testCase.overlayErr) {
				t.Errorf("Health()[overlay] = %v, want %v", got, testCase.overlayErr)
			}
		})
	}
}
