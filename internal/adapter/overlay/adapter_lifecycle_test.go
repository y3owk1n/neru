package overlay_test

import (
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/config"
)

// lifecycleManager records the backend calls the lifecycle half of the port is
// supposed to reach: the screen-share toggle and the one notification a
// configuration change owes the render components.
type lifecycleManager struct {
	headlessManager

	sharingHidden bool
	sharingCalls  int
	configures    int
	configuredFor int
}

func (m *lifecycleManager) SetSharingType(hide bool) {
	m.sharingHidden = hide
	m.sharingCalls++
}

func (m *lifecycleManager) ConfigureComponents(cfg *config.Config, _ overlay.PointerAppearance) {
	m.configures++
	m.configuredFor = cfg.Hints.UI.FontSize
}

// TestAdapterApplyConfig_ReachesTheRenderComponents is the acceptance for the
// half of #1213 a user notices. A config reload or a theme change is one call
// on the port, and it has to arrive at the components the backend draws
// through, carrying the reloaded values. The nil case is the
// degradation an adapter built without a resolver has to make. The app calls
// this on every reload, and it must be inert rather than a daemon that dies
// when the theme changes.
func TestAdapterApplyConfig_ReachesTheRenderComponents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		withResolver   bool
		wantConfigures int
	}{
		{name: "a resolver hands the configuration on", withResolver: true, wantConfigures: 1},
		{name: "no resolver reaches nothing", withResolver: false, wantConfigures: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			manager := &lifecycleManager{}

			var styles overlay.StyleOwner
			if test.withResolver {
				styles = overlay.NewStyleResolver(
					manager,
					config.DefaultConfig(),
					nil,
					zap.NewNop(),
				)
			}

			adapter := overlay.NewAdapter(manager, styles, zap.NewNop())

			reloaded := config.DefaultConfig()
			reloaded.Hints.UI.FontSize = 27

			adapter.ApplyConfig(reloaded)
			adapter.RefreshStyles()

			// A theme change re-resolves from the configuration already held,
			// so a resolver notifies twice for the pair above.
			wantConfigures := test.wantConfigures * 2
			if manager.configures != wantConfigures {
				t.Fatalf(
					"the render components were reconfigured %d times, want %d",
					manager.configures, wantConfigures,
				)
			}

			if test.withResolver && manager.configuredFor != 27 {
				t.Errorf(
					"the render components were handed hints font size %d, want the reloaded 27",
					manager.configuredFor,
				)
			}
		})
	}
}

// TestAdapterSetHiddenInScreenShare_ReachesTheBackend covers the screen-share
// toggle, which is the one overlay state a user drives from the systray and
// the CLI rather than from a mode.
func TestAdapterSetHiddenInScreenShare_ReachesTheBackend(t *testing.T) {
	t.Parallel()

	manager := &lifecycleManager{}
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	adapter.SetHiddenInScreenShare(true)

	if manager.sharingCalls != 1 || !manager.sharingHidden {
		t.Errorf(
			"asked to hide in screen share: %d calls, hidden = %v; want 1 and true",
			manager.sharingCalls, manager.sharingHidden,
		)
	}

	adapter.SetHiddenInScreenShare(false)

	if manager.sharingCalls != 2 || manager.sharingHidden {
		t.Errorf(
			"asked to stop hiding: %d calls, hidden = %v; want 2 and false",
			manager.sharingCalls, manager.sharingHidden,
		)
	}
}
