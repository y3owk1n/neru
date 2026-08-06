package overlay_test

import (
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

// recordingStyles is the Style owner as the adapter depends on it, recording
// which of the two notifications it was given rather than what came out. What
// a re-resolution produces is pinned in style_test.go; what these tests are
// for is that the port's calls arrive at the owner at all.
type recordingStyles struct {
	mu       sync.Mutex
	applied  []*config.Config
	refreshs int
}

func (r *recordingStyles) Style() overlay.Style { return overlay.Style{} }

func (r *recordingStyles) Apply(cfg *config.Config) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.applied = append(r.applied, cfg)
}

func (r *recordingStyles) Refresh() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refreshs++
}

func (r *recordingStyles) counts() (int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.applied), r.refreshs
}

// lifecycleManager records the backend calls the lifecycle half of the port is
// supposed to reach: the screen-share toggle, the shutdown release, and the one
// notification a configuration change owes the render components.
type lifecycleManager struct {
	headlessManager

	sharingHidden bool
	sharingCalls  int
	destroys      int
	configures    int
	configuredFor int
}

func (m *lifecycleManager) SetSharingType(hide bool) {
	m.sharingHidden = hide
	m.sharingCalls++
}

func (m *lifecycleManager) Destroy() { m.destroys++ }

func (m *lifecycleManager) ConfigureComponents(cfg *config.Config, _ string) {
	m.configures++
	m.configuredFor = cfg.Hints.UI.FontSize
}

// capture is what a keyboard grab looks like to a test: which backend was
// asked, and what for. Both managers below point at one, so a table can assert
// on the same value whether or not the backend declares the capability.
type capture struct {
	calls   int
	enabled bool
}

// grabbingManager is a backend whose surface can hold the keyboard, the way
// the Linux ones do.
type grabbingManager struct {
	headlessManager

	seen *capture
}

var _ overlay.KeyboardCaptureController = (*grabbingManager)(nil)

func (m *grabbingManager) SetKeyboardCaptureEnabled(enabled bool) {
	m.seen.calls++
	m.seen.enabled = enabled
}

// deafManager is every other backend: it never takes the keyboard from the
// focused application, so it declares no capability to release it.
type deafManager struct {
	headlessManager
}

// TestAdapterApplyConfig_ReachesTheStyleOwnerOnce is the acceptance for the
// half of #1213 a user notices: a config reload or a theme change is one call
// on the port, and the overlay re-resolves once from it. Before, the app held
// the resolver and the port could not be told at all.
func TestAdapterApplyConfig_ReachesTheStyleOwnerOnce(t *testing.T) {
	t.Parallel()

	styles := &recordingStyles{}
	adapter := overlay.NewAdapter(&headlessManager{}, styles, zap.NewNop())

	reloaded := config.DefaultConfig()
	reloaded.Hints.UI.FontSize = 27

	adapter.ApplyConfig(reloaded)

	applied, refreshed := styles.counts()
	if applied != 1 {
		t.Fatalf("a config reload reached the style owner %d times, want 1", applied)
	}

	if refreshed != 0 {
		t.Errorf("a config reload also asked for %d theme refreshes, want 0", refreshed)
	}

	if styles.applied[0].Hints.UI.FontSize != 27 {
		t.Errorf(
			"the style owner was handed hints font size %d, want the reloaded 27",
			styles.applied[0].Hints.UI.FontSize,
		)
	}

	adapter.RefreshStyles()

	applied, refreshed = styles.counts()
	if applied != 1 || refreshed != 1 {
		t.Errorf(
			"after a theme change the style owner saw %d configs and %d refreshes, want 1 and 1",
			applied, refreshed,
		)
	}
}

// TestAdapterApplyConfig_ReachesTheRenderComponents is the same call end to
// end, over a real resolver: one port call has to arrive at the components the
// backend draws through, carrying the reloaded values. The nil case is the
// degradation an adapter built without a resolver has to make — the app calls
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

// TestAdapterDestroy_ReleasesTheBackend pins the shutdown call. The app used
// to reach the manager for it, which is why the port had to grow it: a daemon
// that exits without releasing the overlay leaves a native window behind.
func TestAdapterDestroy_ReleasesTheBackend(t *testing.T) {
	t.Parallel()

	manager := &lifecycleManager{}
	adapter := overlay.NewAdapter(manager, testStyles{}, zap.NewNop())

	adapter.Destroy()

	if manager.destroys != 1 {
		t.Errorf("Destroy() released the backend %d times, want 1", manager.destroys)
	}
}

// TestAdapterSetKeyboardCaptureEnabled_ReachesOnlyABackendThatGrabs covers
// both halves of the call the mode handler used to make through the overlay
// package's singleton: a Linux backend has a grab to release and hears about
// it, and every other backend never took the keyboard from the focused
// application, so the call is inert rather than a panic.
func TestAdapterSetKeyboardCaptureEnabled_ReachesOnlyABackendThatGrabs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		manager   func(*capture) overlay.ManagerInterface
		wantCalls int
	}{
		{
			name:      "a backend whose surface grabs the keyboard",
			manager:   func(seen *capture) overlay.ManagerInterface { return &grabbingManager{seen: seen} },
			wantCalls: 1,
		},
		{
			name:      "a backend that never took it",
			manager:   func(*capture) overlay.ManagerInterface { return &deafManager{} },
			wantCalls: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			seen := &capture{}

			var port ports.OverlayPort = overlay.NewAdapter(
				test.manager(seen),
				testStyles{},
				zap.NewNop(),
			)

			port.SetKeyboardCaptureEnabled(true)

			if seen.calls != test.wantCalls {
				t.Fatalf("the backend was asked %d times, want %d", seen.calls, test.wantCalls)
			}

			if test.wantCalls > 0 && !seen.enabled {
				t.Error("the backend was asked to release the keyboard, want hold")
			}
		})
	}
}
