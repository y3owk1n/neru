//go:build darwin

package darwin_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/darwin"
	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
)

// TestManager_WithoutAWindowDeclaresItselfHeadless pins what a macOS manager
// reports when it has no overlay window: nothing for the render overlays to
// draw on. The component factory reads that declaration before it builds them,
// and building them against a window that was never created crashes on the
// first CGo call.
//
// A zero-value Manager is exactly that state; Init is deliberately avoided,
// being a process-global singleton that creates a real NSWindow.
func TestManager_WithoutAWindowDeclaresItselfHeadless(t *testing.T) {
	tests := map[string]*darwin.Manager{
		"no window created": {},
		"nil manager":       nil,
	}

	for name, mgr := range tests {
		t.Run(name, func(t *testing.T) {
			var overlayManager manager.Interface = mgr

			reporter, ok := overlayManager.(manager.HeadlessReporter)
			if !ok {
				t.Fatal("the darwin manager does not implement HeadlessReporter")
			}

			if !reporter.Headless() {
				t.Error("Headless() = false; there is no window for an overlay to draw on")
			}
		})
	}
}
