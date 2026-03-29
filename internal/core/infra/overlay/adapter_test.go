package overlay_test

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/overlay"
	"github.com/y3owk1n/neru/internal/core/ports"
	portmocks "github.com/y3owk1n/neru/internal/core/ports/mocks"
	uioverlay "github.com/y3owk1n/neru/internal/ui/overlay"
)

type overlayTestThemeProvider struct{}

func (t *overlayTestThemeProvider) IsDarkMode() bool { return false }

type supportedManager struct {
	uioverlay.NoOpManager
}

func (m *supportedManager) OverlayCapabilities() ports.FeatureCapability {
	return ports.FeatureCapability{
		Status: ports.FeatureStatusSupported,
		Detail: "test overlay available",
	}
}

func TestAdapterHealth_ReturnsNotSupportedForStubOverlayManager(t *testing.T) {
	adapter := overlay.NewAdapter(
		&uioverlay.NoOpManager{},
		&overlayTestThemeProvider{},
		&portmocks.SystemMock{},
		zap.NewNop(),
	)

	err := adapter.Health(context.Background())
	if err == nil {
		t.Fatal("Health() error = nil, want not supported error")
	}
}

func TestAdapterHealth_ReturnsNilForSupportedOverlayManager(t *testing.T) {
	adapter := overlay.NewAdapter(
		&supportedManager{},
		&overlayTestThemeProvider{},
		&portmocks.SystemMock{},
		zap.NewNop(),
	)

	err := adapter.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error = %v, want nil", err)
	}
}
