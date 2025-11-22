package services_test

import (
	"context"
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
				ov.ShowGridFunc = func(ctx context.Context, rows, cols int) error {
					if rows != 3 || cols != 3 {
						t.Errorf("ShowGrid called with rows=%d, cols=%d; want 3, 3", rows, cols)
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
	mockOverlay := &mocks.MockOverlayPort{}
	log := logger.Get()

	service := services.NewGridService(mockOverlay, log)
	ctx := context.Background()

	err := service.HideGrid(ctx)
	if err != nil {
		t.Errorf("HideGrid() unexpected error: %v", err)
	}

	if mockOverlay.IsVisible() {
		t.Error("Overlay should not be visible after HideGrid")
	}
}
