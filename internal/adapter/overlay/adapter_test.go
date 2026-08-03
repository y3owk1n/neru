package overlay_test

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

type overlayTestThemeProvider struct{}

func (t *overlayTestThemeProvider) IsDarkMode() bool { return false }

type supportedManager struct {
	overlay.NoOpManager
}

type stubManager struct {
	overlay.NoOpManager
}

func (m *supportedManager) OverlayCapabilities() ports.FeatureCapability {
	return ports.FeatureCapability{
		Status: ports.FeatureStatusSupported,
		Detail: "test overlay available",
	}
}

func (m *stubManager) OverlayCapabilities() ports.FeatureCapability {
	return ports.FeatureCapability{
		Status: ports.FeatureStatusStub,
		Detail: "test overlay unavailable",
	}
}

func TestAdapterHealth_ReturnsNilForHeadlessOverlayManager(t *testing.T) {
	adapter := overlay.NewAdapter(
		&overlay.NoOpManager{},
		&overlayTestThemeProvider{},
		&portmocks.MockSystemPort{},
		zap.NewNop(),
	)

	err := adapter.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error = %v, want nil", err)
	}
}

func TestAdapterHealth_ReturnsNilForSupportedOverlayManager(t *testing.T) {
	adapter := overlay.NewAdapter(
		&supportedManager{},
		&overlayTestThemeProvider{},
		&portmocks.MockSystemPort{},
		zap.NewNop(),
	)

	err := adapter.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error = %v, want nil", err)
	}
}

func TestAdapterHealth_ReturnsNotSupportedForStubOverlayManager(t *testing.T) {
	adapter := overlay.NewAdapter(
		&stubManager{},
		&overlayTestThemeProvider{},
		&portmocks.MockSystemPort{},
		zap.NewNop(),
	)

	err := adapter.Health(context.Background())
	if err == nil {
		t.Fatal("Health() error = nil, want not supported error")
	}
}
