//go:build linux

package hotkeys

import (
	"testing"

	"go.uber.org/zap"
)

func TestLinuxManagerHealthCheck(t *testing.T) {
	mgr := NewManager(zap.NewNop())
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}

	// Unregistered / zero callbacks should report healthy
	if !mgr.HealthCheck() {
		t.Errorf("expected HealthCheck() to be true when no callbacks registered")
	}
}
