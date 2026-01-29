package services_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
)

// gridStubOverlayPort is a stub for OverlayPort used in grid tests.
type gridStubOverlayPort struct {
	visible      bool
	showGridErr  error
	hideErr      error
	showGridFunc func(context.Context) error
	hideFunc     func(context.Context) error
}

func (s *gridStubOverlayPort) Health(_ context.Context) error { return nil }
func (s *gridStubOverlayPort) ShowHints(_ context.Context, _ []*hint.Interface) error {
	return nil
}

func (s *gridStubOverlayPort) ShowGrid(ctx context.Context) error {
	if s.showGridFunc != nil {
		return s.showGridFunc(ctx)
	}

	return s.showGridErr
}

func (s *gridStubOverlayPort) DrawScrollHighlight(
	_ context.Context,
	_ image.Rectangle,
	_ string,
	_ int,
) error {
	return nil
}

func (s *gridStubOverlayPort) Hide(ctx context.Context) error {
	if s.hideFunc != nil {
		return s.hideFunc(ctx)
	}

	s.visible = false

	return s.hideErr
}
func (s *gridStubOverlayPort) IsVisible() bool { return s.visible }
func (s *gridStubOverlayPort) Refresh(_ context.Context) error {
	return nil
}

func TestGridService_ShowGrid(t *testing.T) {
	tests := []struct {
		name        string
		showGridErr error
		wantErr     bool
	}{
		{
			name:        "successful show",
			showGridErr: nil,
			wantErr:     false,
		},
		{
			name:        "overlay error",
			showGridErr: derrors.New(derrors.CodeOverlayFailed, "overlay initialization failed"),
			wantErr:     true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			stubOv := &gridStubOverlayPort{showGridErr: testCase.showGridErr}
			logger := logger.Get()

			service := services.NewGridService(stubOv, logger)
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
		name    string
		hideErr error
		wantErr bool
	}{
		{
			name:    "successful hide",
			hideErr: nil,
			wantErr: false,
		},
		{
			name:    "overlay hide error",
			hideErr: derrors.New(derrors.CodeOverlayFailed, "failed to hide overlay"),
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			stubOv := &gridStubOverlayPort{visible: true, hideErr: testCase.hideErr}
			logger := logger.Get()

			service := services.NewGridService(stubOv, logger)
			ctx := context.Background()

			hideGridErr := service.HideGrid(ctx)

			if (hideGridErr != nil) != testCase.wantErr {
				t.Errorf("HideGrid() error = %v, wantErr %v", hideGridErr, testCase.wantErr)
			}

			// Only check visibility for successful hide
			if !testCase.wantErr && stubOv.IsVisible() {
				t.Error("Overlay should not be visible after successful HideGrid")
			}
		})
	}
}
