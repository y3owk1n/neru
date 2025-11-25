package services_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/application/ports/mocks"
	"github.com/y3owk1n/neru/internal/application/services"
	derrors "github.com/y3owk1n/neru/internal/errors"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

func TestGridService_ShowGrid(t *testing.T) {
	tests := []struct {
		name       string
		rows       int
		cols       int
		setupMocks func(*mocks.MockOverlayPort)
		wantErr    bool
	}{
		{
			name: "successful show",
			rows: 3,
			cols: 3,
			setupMocks: func(ov *mocks.MockOverlayPort) {
				ov.ShowGridFunc = func(_ context.Context, rows, cols int) error {
					if rows != 3 || cols != 3 {
						t.Errorf("ShowGrid called with rows=%d, cols=%d; want 3, 3", rows, cols)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name: "minimum grid size",
			rows: 1,
			cols: 1,
			setupMocks: func(ov *mocks.MockOverlayPort) {
				ov.ShowGridFunc = func(_ context.Context, rows, cols int) error {
					if rows != 1 || cols != 1 {
						t.Errorf("ShowGrid called with rows=%d, cols=%d; want 1, 1", rows, cols)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name: "large grid size",
			rows: 100,
			cols: 100,
			setupMocks: func(ov *mocks.MockOverlayPort) {
				ov.ShowGridFunc = func(_ context.Context, rows, cols int) error {
					if rows != 100 || cols != 100 {
						t.Errorf("ShowGrid called with rows=%d, cols=%d; want 100, 100", rows, cols)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name: "overlay error",
			rows: 3,
			cols: 3,
			setupMocks: func(ov *mocks.MockOverlayPort) {
				ov.ShowGridFunc = func(_ context.Context, _, _ int) error {
					return derrors.New(
						derrors.CodeOverlayFailed,
						"overlay initialization failed",
					)
				}
			},
			wantErr: true,
		},
		{
			name: "asymmetric grid",
			rows: 5,
			cols: 10,
			setupMocks: func(ov *mocks.MockOverlayPort) {
				ov.ShowGridFunc = func(_ context.Context, rows, cols int) error {
					if rows != 5 || cols != 10 {
						t.Errorf("ShowGrid called with rows=%d, cols=%d; want 5, 10", rows, cols)
					}

					return nil
				}
			},
			wantErr: false,
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
			context := context.Background()

			showGridErr := service.ShowGrid(context, testCase.rows, testCase.cols)

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
			context := context.Background()

			hideGridErr := service.HideGrid(context)

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
