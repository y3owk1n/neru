//go:build !darwin

package manager_test

// The full build is asserted off macOS only: there the mode indicator, sticky
// indicator and virtual pointer each create their own native window, which is
// a real desktop resource and not something a unit test should allocate. What
// they are built from — configuration and a theme provider — is shared, so the
// wiring this pins is the same wiring macOS runs.

import (
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

// TestBase_BuildComponents_BuildsWhatTheConfigurationAsksFor pins that the
// overlay builds its own components from the configuration and theme it is
// handed, and that the two grid overlays exist even with their modes disabled
// — enabling one through a config reload must not need a restart to get an
// overlay. Hints are the exception: the mode being off means no overlay.
func TestBase_BuildComponents_BuildsWhatTheConfigurationAsksFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		hints      bool
		grid       bool
		recursive  bool
		wantHints  bool
		wantGrid   bool
		wantRecurs bool
	}{
		{
			name: "every mode enabled", hints: true, grid: true, recursive: true,
			wantHints: true, wantGrid: true, wantRecurs: true,
		},
		{
			name: "every mode disabled", hints: false, grid: false, recursive: false,
			wantHints: false, wantGrid: true, wantRecurs: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			cfg.Hints.Enabled = testCase.hints
			cfg.Grid.Enabled = testCase.grid
			cfg.RecursiveGrid.Enabled = testCase.recursive

			base := manager.NewBase(nil)

			built, err := base.BuildComponents(manager.ComponentSpec{
				Config: cfg,
				Theme:  lightTheme{},
				Logger: zap.NewNop(),
			})
			if err != nil {
				t.Fatalf("BuildComponents() error = %v, want nil", err)
			}

			if (built.Hints != nil) != testCase.wantHints {
				t.Errorf(
					"hints overlay built = %t, want %t",
					built.Hints != nil,
					testCase.wantHints,
				)
			}

			if (built.Grid != nil) != testCase.wantGrid {
				t.Errorf("grid overlay built = %t, want %t", built.Grid != nil, testCase.wantGrid)
			}

			if (built.RecursiveGrid != nil) != testCase.wantRecurs {
				t.Errorf(
					"recursive-grid overlay built = %t, want %t",
					built.RecursiveGrid != nil,
					testCase.wantRecurs,
				)
			}

			if built.ModeIndicator == nil {
				t.Error("no mode indicator overlay was built")
			}

			if built.StickyModifiers == nil {
				t.Error("no sticky modifiers indicator overlay was built")
			}

			if built.VirtualPointer == nil {
				t.Error("no virtual pointer overlay was built")
			}
		})
	}
}

// TestBase_BuildComponents_RegistersWhatItBuilt pins that the manager keeps
// the components as well as returning them: an indicator is driven through the
// port, and the manager can only resolve one to a render component if the
// build registered it.
func TestBase_BuildComponents_RegistersWhatItBuilt(t *testing.T) {
	t.Parallel()

	base := manager.NewBase(nil)

	built, err := base.BuildComponents(manager.ComponentSpec{
		Config: config.DefaultConfig(),
		Theme:  lightTheme{},
		Logger: zap.NewNop(),
	})
	if err != nil {
		t.Fatalf("BuildComponents() error = %v, want nil", err)
	}

	if base.ModeIndicatorOverlay() != built.ModeIndicator {
		t.Error("the mode indicator it built is not the one it kept")
	}

	if base.GridOverlay() != built.Grid {
		t.Error("the grid overlay it built is not the one it kept")
	}

	if base.VirtualPointerOverlay() != built.VirtualPointer {
		t.Error("the virtual pointer it built is not the one it kept")
	}

	// Reached through the port vocabulary, which is how every caller reaches
	// one: a registered indicator answers, rather than being skipped as never
	// constructed.
	base.ShowIndicator(ports.ModeIndicator)
	base.HideIndicator(ports.ModeIndicator)
}
