package overlay_test

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/ports"
)

// testStyles is a StyleSource for the health tests, which never draw.
type testStyles struct{}

func (testStyles) Style() overlay.Style { return overlay.Style{} }

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
		testStyles{},
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
		testStyles{},
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
		testStyles{},
		zap.NewNop(),
	)

	err := adapter.Health(context.Background())
	if err == nil {
		t.Fatal("Health() error = nil, want not supported error")
	}
}
