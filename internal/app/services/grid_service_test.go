//go:build unit

package services_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/ports/mocks"
)

func TestGridService_ShowGrid(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(*mocks.MockOverlayPort)
		wantErr    bool
	}{
		{
			name: "successful show",
			setupMocks: func(ov *mocks.MockOverlayPort) {
				ov.ShowGridFunc = func(_ context.Context) error {
					return nil
				}
			},
			wantErr: false,
		},
		{
			name: "overlay error",
			setupMocks: func(ov *mocks.MockOverlayPort) {
				ov.ShowGridFunc = func(_ context.Context) error {
					return derrors.New(
						derrors.CodeOverlayFailed,
						"overlay initialization failed",
					)
				}
			},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockOverlay := &mocks.MockOverlayPort{}
			logger := logger.Get()

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockOverlay)
			}

			service := services.NewGridService(mockOverlay, logger)
			ctx := context.Background()

			showGridErr := service.ShowGrid(ctx)

			if (showGridErr != nil) != testCase.wantErr {
				t.Errorf("ShowGrid() error = %v, wantErr %v", showGridErr, testCase.wantErr)
			}
		})
	}
}

func TestGridService_HideGrid(t *testing.T) {
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
			mockOverlay := &mocks.MockOverlayPort{}
			logger := logger.Get()

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockOverlay)
			}

			service := services.NewGridService(mockOverlay, logger)
			ctx := context.Background()

			hideGridErr := service.HideGrid(ctx)

			if (hideGridErr != nil) != testCase.wantErr {
				t.Errorf("HideGrid() error = %v, wantErr %v", hideGridErr, testCase.wantErr)
			}

			// Only check visibility for successful hide
			if !testCase.wantErr && mockOverlay.IsVisible() {
				t.Error("Overlay should not be visible after successful HideGrid")
			}
		})
	}
}
