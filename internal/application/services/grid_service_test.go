package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/y3owk1n/neru/internal/application/ports/mocks"
	"github.com/y3owk1n/neru/internal/application/services"
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
					return errors.New("overlay initialization failed")
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOverlay := &mocks.MockOverlayPort{}
			log := logger.Get()

			if tt.setupMocks != nil {
				tt.setupMocks(mockOverlay)
			}

			service := services.NewGridService(mockOverlay, log)
			ctx := context.Background()

			err := service.ShowGrid(ctx, tt.rows, tt.cols)

			if (err != nil) != tt.wantErr {
				t.Errorf("ShowGrid() error = %v, wantErr %v", err, tt.wantErr)
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
					return errors.New("failed to hide overlay")
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOverlay := &mocks.MockOverlayPort{}
			log := logger.Get()

			if tt.setupMocks != nil {
				tt.setupMocks(mockOverlay)
			}

			service := services.NewGridService(mockOverlay, log)
			ctx := context.Background()

			err := service.HideGrid(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("HideGrid() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Only check visibility for successful hide
			if !tt.wantErr && mockOverlay.IsVisible() {
				t.Error("Overlay should not be visible after successful HideGrid")
			}
		})
	}
}
