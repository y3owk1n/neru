package overlay_test

import (
	"context"
	"image"
	"sync"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	renderhints "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
)

// countingTheme records every time a Style resolution asks what the system
// appearance is, which is the only way resolution can be observed from
// outside.
type countingTheme struct {
	dark  atomic.Bool
	reads atomic.Int64
}

func (t *countingTheme) IsDarkMode() bool {
	t.reads.Add(1)

	return t.dark.Load()
}

// TestStyleResolver_ResolvesPerChangeNotPerDraw pins the decision that makes
// this ownership move free: a Style is resolved when the config or the theme
// changes, and a draw reads the cached result. If a draw ever resolved again
// the overlay would pay a theme lookup and a fresh allocation per frame.
func TestStyleResolver_ResolvesPerChangeNotPerDraw(t *testing.T) {
	t.Parallel()

	theme := &countingTheme{}
	cfg := config.DefaultConfig()
	manager := &overlay.NoOpManager{}

	resolver := overlay.NewStyleResolver(manager, cfg, theme, zap.NewNop())
	adapter := overlay.NewAdapter(manager, resolver, zap.NewNop())

	target, elementErr := element.NewElement("e1", image.Rect(0, 0, 10, 10), "button")
	if elementErr != nil {
		t.Fatalf("NewElement() error = %v", elementErr)
	}

	drawn, hintErr := hint.NewHint("A", target, image.Pt(0, 0))
	if hintErr != nil {
		t.Fatalf("NewHint() error = %v", hintErr)
	}

	hints := []*hint.Interface{drawn}

	readsAfterConstruction := theme.reads.Load()
	if readsAfterConstruction == 0 {
		t.Fatal("constructing the resolver never consulted the theme")
	}

	for range 10 {
		_ = resolver.Style()

		showErr := adapter.ShowHints(context.Background(), hints)
		if showErr != nil {
			t.Fatalf("ShowHints() error = %v", showErr)
		}
	}

	if got := theme.reads.Load(); got != readsAfterConstruction {
		t.Errorf(
			"theme reads after 10 draws = %d, want %d: Style is being resolved per draw",
			got,
			readsAfterConstruction,
		)
	}

	resolver.Apply(cfg)

	if got := theme.reads.Load(); got <= readsAfterConstruction {
		t.Errorf(
			"theme reads after a config change = %d, want more than %d: the config change resolved nothing",
			got,
			readsAfterConstruction,
		)
	}

	readsAfterApply := theme.reads.Load()

	resolver.Refresh()

	if got := theme.reads.Load(); got <= readsAfterApply {
		t.Errorf(
			"theme reads after a theme change = %d, want more than %d: the theme change resolved nothing",
			got,
			readsAfterApply,
		)
	}
}

// TestStyleResolver_RefreshPicksUpTheNewTheme is the user-facing half: the
// same configuration resolves to different colors once the system appearance
// flips, and nothing but the one notification is needed to get there.
func TestStyleResolver_RefreshPicksUpTheNewTheme(t *testing.T) {
	t.Parallel()

	theme := &countingTheme{}
	cfg := config.DefaultConfig()

	resolver := overlay.NewStyleResolver(&overlay.NoOpManager{}, cfg, theme, zap.NewNop())

	light := resolver.Style()

	theme.dark.Store(true)

	if got := resolver.Style().Hints.TextColor(); got != light.Hints.TextColor() {
		t.Fatalf(
			"Style changed before Refresh: %q, want the cached %q",
			got,
			light.Hints.TextColor(),
		)
	}

	resolver.Refresh()

	dark := resolver.Style()

	if dark.Hints.TextColor() == light.Hints.TextColor() {
		t.Errorf("hints text color = %q in both themes", dark.Hints.TextColor())
	}

	if dark.Grid.TextColor() == light.Grid.TextColor() {
		t.Errorf("grid text color = %q in both themes", dark.Grid.TextColor())
	}

	if dark.RecursiveGrid.TextColor() == light.RecursiveGrid.TextColor() {
		t.Errorf("recursive-grid text color = %q in both themes", dark.RecursiveGrid.TextColor())
	}

	if dark.MonitorSelect.TextColor == light.MonitorSelect.TextColor {
		t.Errorf("monitor-select text color = %q in both themes", dark.MonitorSelect.TextColor)
	}

	if dark.HintSearchInput.TextColor() == light.HintSearchInput.TextColor() {
		t.Errorf(
			"hint search input text color = %q in both themes",
			dark.HintSearchInput.TextColor(),
		)
	}

	if dark.VirtualPointer.FillColor == light.VirtualPointer.FillColor {
		t.Errorf("virtual pointer fill color = %q in both themes", dark.VirtualPointer.FillColor)
	}
}

// TestStyleResolver_ZeroValueTheme pins that a resolver built without a theme
// provider still resolves rather than panicking; a headless start has no
// appearance to report.
func TestStyleResolver_ZeroValueTheme(t *testing.T) {
	t.Parallel()

	resolver := overlay.NewStyleResolver(nil, nil, nil, nil)

	if resolver.Style().Hints.TextColor() == "" {
		t.Error("Style() resolved no hints text color from the default config")
	}

	resolver.Refresh()

	if resolver.Style().Grid.TextColor() == "" {
		t.Error("Refresh() left no grid text color resolved")
	}
}

// styleTestManager holds one hints overlay so a test can see what a config
// notification handed it. Every other component stays nil, which is what a
// partially wired manager looks like during startup.
type styleTestManager struct {
	overlay.NoOpManager

	hints *renderhints.Overlay
}

func (m *styleTestManager) HintOverlay() *renderhints.Overlay { return m.hints }

// TestStyleResolver_ApplyLeavesDisabledOverlaysAlone pins the gate the
// per-component reload path used to carry: a disabled overlay draws nothing,
// so reconfiguring it would only invalidate caches nobody reads.
func TestStyleResolver_ApplyLeavesDisabledOverlaysAlone(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true
	cfg.Hints.UI.FontSize = 11

	hintOverlay, err := renderhints.NewOverlayWithWindow(cfg.Hints, zap.NewNop(), nil)
	if err != nil {
		t.Fatalf("NewOverlayWithWindow() error = %v", err)
	}

	manager := &styleTestManager{hints: hintOverlay}
	resolver := overlay.NewStyleResolver(manager, cfg, &countingTheme{}, zap.NewNop())

	enabled := config.DefaultConfig()
	enabled.Hints.Enabled = true
	enabled.Hints.UI.FontSize = 33

	resolver.Apply(enabled)

	if got := hintOverlay.Config().UI.FontSize; got != 33 {
		t.Fatalf("hints overlay font size after an enabled reload = %d, want 33", got)
	}

	disabled := config.DefaultConfig()
	disabled.Hints.Enabled = false
	disabled.Hints.UI.FontSize = 44

	resolver.Apply(disabled)

	if got := hintOverlay.Config().UI.FontSize; got != 33 {
		t.Errorf("a disabled hints overlay was reconfigured: font size = %d, want 33", got)
	}

	// The Style still follows the reload — only the push is gated, because a
	// mode that is re-enabled later must not draw with pre-reload colors.
	if got := resolver.Style().Hints.FontSize(); got != 44 {
		t.Errorf("resolved hints font size = %d, want 44", got)
	}
}

// TestStyleResolver_ConcurrentApplyAndRead drives the two writers the daemon
// really has — a config reload and a theme observer — against the readers
// every draw is, under -race. A draw must never observe a half-published
// Style, and a theme change must never republish a configuration a reload has
// already replaced.
func TestStyleResolver_ConcurrentApplyAndRead(t *testing.T) {
	t.Parallel()

	reloaded := config.DefaultConfig()
	reloaded.Hints.UI.FontSize = 41

	resolver := overlay.NewStyleResolver(
		&overlay.NoOpManager{},
		config.DefaultConfig(),
		&countingTheme{},
		zap.NewNop(),
	)

	var waiter sync.WaitGroup

	waiter.Add(3)

	go func() {
		defer waiter.Done()

		for range 200 {
			resolver.Apply(reloaded)
		}
	}()

	go func() {
		defer waiter.Done()

		for range 200 {
			resolver.Refresh()
		}
	}()

	go func() {
		defer waiter.Done()

		for range 200 {
			_ = resolver.Style()
		}
	}()

	waiter.Wait()

	// Every reload applied the same config, so whichever writer went last, the
	// resolved Style must be the reloaded one — a Refresh that snapshotted the
	// config before a reload and applied it afterwards would leave the old
	// value here.
	if got := resolver.Style().Hints.FontSize(); got != 41 {
		t.Errorf("resolved hints font size = %d, want 41: a theme change reverted a reload", got)
	}
}

// TestResolvedStyle_MissingSource covers the two shapes of "no source": a
// consumer built without one, and a typed nil resolver stored in the
// interface, which passes every != nil guard a caller could write.
func TestResolvedStyle_MissingSource(t *testing.T) {
	t.Parallel()

	if got := overlay.ResolvedStyle(nil); got != (overlay.Style{}) {
		t.Errorf("ResolvedStyle(nil) = %+v, want the zero Style", got)
	}

	var typedNil *overlay.StyleResolver

	if got := overlay.ResolvedStyle(typedNil); got != (overlay.Style{}) {
		t.Errorf("ResolvedStyle(typed nil) = %+v, want the zero Style", got)
	}
}
