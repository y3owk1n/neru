//go:build darwin

package darwin_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/darwin"
	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/config"
)

// TestManager_WithoutAWindowDeclaresItselfHeadless pins what a macOS manager
// reports when it has no overlay window: nothing for the render overlays to
// draw on. Its own component build reads that declaration before it builds
// them, and building them against a window that was never created crashes on
// the first CGo call.
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

			// The declaration has to be what building reads. A backend that
			// re-derived it — from a window handle it can see, say — would
			// still pass the assertion above and then build overlays on a
			// window that was never created.
			built, err := overlayManager.BuildComponents(config.DefaultConfig(), nil)
			if err != nil {
				t.Fatalf("BuildComponents() error = %v, want nil", err)
			}

			if built != (manager.Components{}) {
				t.Errorf(
					"BuildComponents() = %+v, want nothing built: the manager declared itself headless",
					built,
				)
			}
		})
	}
}
