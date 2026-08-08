package manager_test

import (
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	renderhints "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

// testPointer is a resolved virtual-pointer appearance for the notifications
// these tests send. Its values are arbitrary; what they stand for is that a
// caller always has every one of them settled by the time it notifies.
var testPointer = manager.PointerAppearance{
	FillColor:  "#ffffff",
	FontFamily: "Test Sans",
	Char:       "x",
	FontSize:   9,
}

// lightTheme is a fixed appearance, so building a component needs no system.
type lightTheme struct{}

func (lightTheme) IsDarkMode() bool { return false }

// TestBase_BuildComponents_BuildsNothingWhenHeadless pins the guard that used
// to live in the app's component factory: a manager with no surface builds no
// render components, because every draw through one reaches a native API
// (CGo, X11) that was never opened. Headlessness is stated by the backend, not
// inferred here from a window handle that happens to be visible.
func TestBase_BuildComponents_BuildsNothingWhenHeadless(t *testing.T) {
	t.Parallel()

	base := manager.NewBase(nil)

	built, err := base.BuildComponents(manager.ComponentSpec{
		Config:   config.DefaultConfig(),
		Theme:    lightTheme{},
		Logger:   zap.NewNop(),
		Headless: true,
	})
	if err != nil {
		t.Fatalf("BuildComponents() error = %v, want nil", err)
	}

	if built != (manager.Components{}) {
		t.Errorf("BuildComponents() = %+v, want nothing built for a headless manager", built)
	}

	// Nothing was registered either, so an indicator call still finds no
	// surface rather than a half-built one.
	for _, indicator := range []ports.Indicator{
		ports.ModeIndicator,
		ports.StickyModifiersIndicator,
		ports.VirtualPointerIndicator,
	} {
		base.ShowIndicator(indicator)
	}
}

// TestBase_ConfigureComponents_LeavesDisabledOverlaysAlone pins the gate the
// per-component reload path used to carry: a disabled overlay draws nothing,
// so reconfiguring it would only invalidate caches nobody reads.
func TestBase_ConfigureComponents_LeavesDisabledOverlaysAlone(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true
	cfg.Hints.UI.FontSize = 11

	// Built directly rather than through BuildComponents: the hints overlay
	// shares the manager's window, so it needs no surface of its own.
	hintOverlay, err := renderhints.NewOverlayWithWindow(cfg.Hints, zap.NewNop(), nil)
	if err != nil {
		t.Fatalf("NewOverlayWithWindow() error = %v", err)
	}

	base := manager.NewBase(nil)
	base.UseHintOverlay(hintOverlay)

	enabled := config.DefaultConfig()
	enabled.Hints.Enabled = true
	enabled.Hints.UI.FontSize = 33

	base.ConfigureComponents(enabled, testPointer)

	if got := hintOverlay.Config().UI.FontSize; got != 33 {
		t.Fatalf("hints overlay font size after an enabled reload = %d, want 33", got)
	}

	disabled := config.DefaultConfig()
	disabled.Hints.Enabled = false
	disabled.Hints.UI.FontSize = 44

	base.ConfigureComponents(disabled, testPointer)

	if got := hintOverlay.Config().UI.FontSize; got != 33 {
		t.Errorf("a disabled hints overlay was reconfigured: font size = %d, want 33", got)
	}
}

// TestBase_ConfigureComponents_WithoutComponents pins that the notification is
// silent before anything is built. A headless start and a failed build both
// leave the registry empty, and the config reload path runs regardless — it
// must neither panic nor conjure a component to configure.
func TestBase_ConfigureComponents_WithoutComponents(t *testing.T) {
	t.Parallel()

	base := manager.NewBase(nil)

	base.ConfigureComponents(config.DefaultConfig(), testPointer)
	base.ConfigureComponents(nil, manager.PointerAppearance{})

	if got := base.GridOverlay(); got != nil {
		t.Errorf("GridOverlay() = %v, want nil: a notification registered a component", got)
	}

	if got := base.ModeIndicatorOverlay(); got != nil {
		t.Errorf(
			"ModeIndicatorOverlay() = %v, want nil: a notification registered a component",
			got,
		)
	}
}
