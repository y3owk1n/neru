package services_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/ports/mocks"
)

func TestScrollService_ScrollDelta(t *testing.T) {
	tests := []struct {
		name       string
		deltaX     int
		deltaY     int
		setupMocks func(*mocks.MockAccessibilityPort)
		wantErr    bool
	}{
		{
			name:   "scroll down",
			deltaX: 0,
			deltaY: -50,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, deltaY int) error {
					if deltaX != 0 {
						t.Errorf("Expected deltaX=0, got %d", deltaX)
					}

					if deltaY != -50 {
						t.Errorf("Expected deltaY=-50, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:   "scroll up",
			deltaX: 0,
			deltaY: 50,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, deltaY int) error {
					if deltaX != 0 {
						t.Errorf("Expected deltaX=0, got %d", deltaX)
					}

					if deltaY != 50 {
						t.Errorf("Expected deltaY=50, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:   "scroll left",
			deltaX: 50,
			deltaY: 0,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, deltaY int) error {
					if deltaX != 50 {
						t.Errorf("Expected deltaX=50, got %d", deltaX)
					}

					if deltaY != 0 {
						t.Errorf("Expected deltaY=0, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:   "scroll right",
			deltaX: -50,
			deltaY: 0,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, deltaY int) error {
					if deltaX != -50 {
						t.Errorf("Expected deltaX=-50, got %d", deltaX)
					}

					if deltaY != 0 {
						t.Errorf("Expected deltaY=0, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:   "accessibility error",
			deltaX: 0,
			deltaY: -50,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, _ int) error {
					return derrors.New(
						derrors.CodeAccessibilityFailed,
						"scroll permission denied",
					)
				}
			},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			logger := logger.Get()

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockAcc)
			}

			service := services.NewScrollService(
				mockAcc,
				mockOverlay,
				&mocks.SystemMock{},
				logger,
			)
			ctx := context.Background()

			scrollErr := service.ScrollDelta(ctx, testCase.deltaX, testCase.deltaY)

			if (scrollErr != nil) != testCase.wantErr {
				t.Errorf("ScrollDelta() error = %v, wantErr %v", scrollErr, testCase.wantErr)
			}
		})
	}
}

func TestScrollService_Hide(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(*mocks.MockOverlayPort)
		wantErr    bool
	}{
		{
			name: "successful hide",
			setupMocks: func(ov *mocks.MockOverlayPort) {
				ov.HideFunc = func(_ context.Context) error {
					return nil
				}
			},
			wantErr: false,
		},
		{
			name: "overlay hide error",
			setupMocks: func(ov *mocks.MockOverlayPort) {
				ov.HideFunc = func(_ context.Context) error {
					return derrors.New(
						derrors.CodeOverlayFailed,
						"failed to hide overlay",
					)
				}
			},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			logger := logger.Get()

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockOverlay)
			}

			service := services.NewScrollService(
				mockAcc,
				mockOverlay,
				&mocks.SystemMock{},
				logger,
			)
			ctx := context.Background()

			hideScrollOverlayErr := service.Hide(ctx)

			if (hideScrollOverlayErr != nil) != testCase.wantErr {
				t.Errorf(
					"Hide() error = %v, wantErr %v",
					hideScrollOverlayErr,
					testCase.wantErr,
				)
			}
		})
	}
}
