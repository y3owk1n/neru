package accessibility_test

import (
	"context"
	"runtime"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/accessibility"
)

// newTestAdapter builds an adapter over the mock AX client with no exclusions.
func newTestAdapter(t *testing.T) *accessibility.Adapter {
	t.Helper()

	return accessibility.NewAdapter(
		zap.NewNop(),
		nil,
		&accessibility.MockAXClient{},
		false,
	)
}

// TestAdapter_PrimeApplication covers the port method that replaced the
// free-function electron.EnsureAccessibility.
//
// The contract forbids an error here, CodeNotSupported included, because
// "nothing to do" counts as success. Eager backends report ready at once. On
// darwin the call polls the real AX tree, so ready depends on the host and the
// test does not check it there.
func TestAdapter_PrimeApplication(t *testing.T) {
	t.Parallel()

	adapter := newTestAdapter(t)

	ready, err := adapter.PrimeApplication(t.Context(), "com.example.app")
	if err != nil {
		t.Fatalf("PrimeApplication() error = %v, want nil", err)
	}

	if runtime.GOOS != "darwin" && !ready {
		t.Error("PrimeApplication() ready = false, want true on an eager backend")
	}
}

// TestAdapter_PrimeApplicationHonorsCanceledContext pins that the retry loop in
// the app's focus-change path cannot spin after shutdown.
func TestAdapter_PrimeApplicationHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	adapter := newTestAdapter(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := adapter.PrimeApplication(ctx, "com.example.app")
	if err == nil {
		t.Fatal("PrimeApplication() on a canceled context error = nil, want an error")
	}
}

// TestAdapter_ReleaseHeldButtons covers the port method that replaced the
// exported accessibility.EnsureMouseUp free function.
//
// It runs on every mode-exit path, so it must be idempotent and safe when
// nothing is held.
func TestAdapter_ReleaseHeldButtons(t *testing.T) {
	t.Parallel()

	adapter := newTestAdapter(t)

	for attempt := range 3 {
		err := adapter.ReleaseHeldButtons(t.Context())
		if err != nil {
			t.Fatalf("ReleaseHeldButtons() attempt %d error = %v, want nil", attempt+1, err)
		}
	}
}

// TestAdapter_ReleaseHeldButtonsHonorsCanceledContext pins the context check,
// so a canceled cleanup does not reach into the platform layer.
func TestAdapter_ReleaseHeldButtonsHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	adapter := newTestAdapter(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := adapter.ReleaseHeldButtons(ctx)
	if err == nil {
		t.Fatal("ReleaseHeldButtons() on a canceled context error = nil, want an error")
	}
}
