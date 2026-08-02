//go:build linux

package hotkeys_test

import (
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/hotkeys"
)

func TestLinuxManagerHealthCheck(t *testing.T) {
	mgr := hotkeys.NewManager(zap.NewNop())
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}

	// Unregistered / zero callbacks should report healthy
	if !mgr.HealthCheck() {
		t.Errorf("expected HealthCheck() to be true when no callbacks registered")
	}
}
