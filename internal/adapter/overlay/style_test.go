package overlay_test

import (
	"context"
	"image"
	"sync"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/ports"
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
	manager := &headlessManager{}

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

	frame := ports.HintsFrame{Screen: image.Rect(0, 0, 100, 100), Hints: hints}

	for range 10 {
		_ = resolver.Style()

		showErr := adapter.ShowFrame(context.Background(), frame)
		if showErr != nil {
			t.Fatalf("ShowFrame() error = %v", showErr)
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

	resolver := overlay.NewStyleResolver(&headlessManager{}, cfg, theme, zap.NewNop())

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

// styleTestManager records the configuration notifications it is handed. The
// components themselves are the manager's own, so what a resolver can be held
// to is that it notifies once per change and carries the resolved value the
// components cannot derive.
type styleTestManager struct {
	headlessManager

	mu       sync.Mutex
	pointers []overlay.PointerAppearance
	sizes    []int
}

func (m *styleTestManager) ConfigureComponents(
	cfg *config.Config,
	pointer overlay.PointerAppearance,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pointers = append(m.pointers, pointer)
	m.sizes = append(m.sizes, cfg.Hints.UI.FontSize)
}

func (m *styleTestManager) notifications() ([]overlay.PointerAppearance, []int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]overlay.PointerAppearance(nil), m.pointers...),
		append([]int(nil), m.sizes...)
}

// TestStyleResolver_ApplyNotifiesTheComponentsOnce pins what a config reload
// owes the render components: exactly one notification carrying the new
// configuration and the resolved value they cannot resolve for themselves.
// Fanning out to individual overlays is what this replaced, and a missed call
// site left an overlay drawing the previous theme's colors.
func TestStyleResolver_ApplyNotifiesTheComponentsOnce(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.UI.FontSize = 11

	manager := &styleTestManager{}
	resolver := overlay.NewStyleResolver(manager, cfg, &countingTheme{}, zap.NewNop())

	if pointers, _ := manager.notifications(); len(pointers) != 0 {
		t.Fatalf("constructing the resolver notified %d times, want 0", len(pointers))
	}

	reloaded := config.DefaultConfig()
	reloaded.Hints.UI.FontSize = 33

	resolver.Apply(reloaded)

	pointers, sizes := manager.notifications()
	if len(pointers) != 1 {
		t.Fatalf("a config reload notified %d times, want 1", len(pointers))
	}

	if sizes[0] != 33 {
		t.Errorf("notified hints font size = %d, want the reloaded 33", sizes[0])
	}

	if pointers[0].FillColor != resolver.Style().VirtualPointer.FillColor {
		t.Errorf(
			"notified virtual pointer fill = %q, want the resolved %q",
			pointers[0].FillColor,
			resolver.Style().VirtualPointer.FillColor,
		)
	}

	resolver.Refresh()

	pointers, sizes = manager.notifications()
	if len(pointers) != 2 {
		t.Fatalf("a theme change notified %d times in total, want 2", len(pointers))
	}

	// A theme change carries the configuration the resolver already holds, not
	// the one it was constructed with.
	if sizes[1] != 33 {
		t.Errorf("theme-change notification carried font size %d, want the reloaded 33", sizes[1])
	}

	// Resolving is never gated, even though handing the configuration on can
	// be: a mode that is disabled now and re-enabled later must not draw with
	// pre-reload colors.
	disabled := config.DefaultConfig()
	disabled.Hints.Enabled = false
	disabled.Hints.UI.FontSize = 44

	resolver.Apply(disabled)

	if got := resolver.Style().Hints.FontSize(); got != 44 {
		t.Errorf("resolved hints font size = %d, want 44 from the reload that disabled hints", got)
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
		&headlessManager{},
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

// markerFontResolver stands in for a platform font resolver. It answers with a
// marker around the name it was asked, so a family that reached it can be told
// apart from one copied straight out of the configuration.
type markerFontResolver struct{}

func (markerFontResolver) Resolve(family string) string {
	return "resolved(" + family + ")"
}

// writtenAlias is a generic alias in a spelling no font engine understands
// literally, and resolvedAlias is what markerFontResolver answers it with. A
// family that still reads as the written one never reached the resolver.
const (
	writtenAlias  = "sans_serif"
	resolvedAlias = "resolved(" + writtenAlias + ")"
)

// TestStyleResolver_RoutesItsFontFamiliesThroughTheResolver pins the three
// families that used to be copied out of the configuration verbatim, so a
// generic alias reached the platform as a family name nothing is installed
// under and the overlay silently fell back to the system font (#1305). Every
// other overlay's family goes through the shared resolver; these must too.
//
// Not parallel: the font resolver is process-wide.
func TestStyleResolver_RoutesItsFontFamiliesThroughTheResolver(t *testing.T) {
	ports.SetFontResolver(markerFontResolver{})
	t.Cleanup(func() { ports.SetFontResolver(nil) })

	cfg := config.DefaultConfig()
	cfg.MonitorSelect.UI.FontFamily = writtenAlias
	cfg.MonitorSelect.UI.SubtitleFontFamily = "mono space"
	cfg.VirtualPointer.UI.FontFamily = "sans-serif"

	style := overlay.NewStyleResolver(
		&headlessManager{}, cfg, &countingTheme{}, zap.NewNop(),
	).Style()

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"monitor-select label", style.MonitorSelect.FontFamily, resolvedAlias},
		{"monitor-select subtitle", style.MonitorSelect.SubtitleFontFamily, "resolved(mono space)"},
		{"virtual pointer", style.VirtualPointer.FontFamily, "resolved(sans-serif)"},
	}

	for _, testCase := range cases {
		if testCase.got != testCase.want {
			t.Errorf(
				"%s font family = %q, want %q: the configured family never reached the resolver",
				testCase.name,
				testCase.got,
				testCase.want,
			)
		}
	}
}

// TestStyleResolver_NotifiesTheComponentsOfTheResolvedPointerFamily pins the
// other half of the same fix: the components that draw the virtual pointer
// read the family out of the configuration they are handed, so the resolved
// one has to travel with the notification. Without it macOS keeps drawing the
// written name — the alias never reaches a face, and the platform falls back
// to the system font (#1305).
//
// Not parallel: the font resolver is process-wide.
func TestStyleResolver_NotifiesTheComponentsOfTheResolvedPointerFamily(t *testing.T) {
	ports.SetFontResolver(markerFontResolver{})
	t.Cleanup(func() { ports.SetFontResolver(nil) })

	cfg := config.DefaultConfig()
	cfg.VirtualPointer.UI.FontFamily = writtenAlias

	manager := &styleTestManager{}
	resolver := overlay.NewStyleResolver(
		manager,
		config.DefaultConfig(),
		&countingTheme{},
		zap.NewNop(),
	)

	resolver.Apply(cfg)

	pointers, _ := manager.notifications()
	if len(pointers) != 1 {
		t.Fatalf("a config reload notified %d times, want 1", len(pointers))
	}

	if got := pointers[0].FontFamily; got != resolvedAlias {
		t.Errorf(
			"notified virtual pointer font family = %q, want the resolved %q",
			got,
			resolvedAlias,
		)
	}
}

// TestStyleResolver_NotifiesTheComponentsOfTheSettledPointerCharAndSize pins
// the same propagation for the two fields a user can leave empty. The resolver
// falls back to the documented `●` at 8pt for them, and the Linux and Windows
// draws see that because they read the Style; the macOS components read char
// and font size out of the configuration they are handed, so an explicitly
// empty char or zero font size drew nothing at no size there (#1337). The
// settled values have to travel with the notification, like the family.
func TestStyleResolver_NotifiesTheComponentsOfTheSettledPointerCharAndSize(t *testing.T) {
	t.Parallel()

	// Written empty on purpose: omitting the keys is filled in by the config
	// defaults, so only an explicit empty string or zero reaches the resolver.
	cfg := config.DefaultConfig()
	cfg.VirtualPointer.UI.Char = ""
	cfg.VirtualPointer.UI.FontSize = 0

	manager := &styleTestManager{}
	resolver := overlay.NewStyleResolver(
		manager,
		config.DefaultConfig(),
		&countingTheme{},
		zap.NewNop(),
	)

	resolver.Apply(cfg)

	pointers, _ := manager.notifications()
	if len(pointers) != 1 {
		t.Fatalf("a config reload notified %d times, want 1", len(pointers))
	}

	if got := pointers[0].Char; got != config.DefaultVirtualPointerChar {
		t.Errorf(
			"notified virtual pointer char = %q, want the settled default %q",
			got,
			config.DefaultVirtualPointerChar,
		)
	}

	if got := pointers[0].FontSize; got != config.DefaultVirtualPointerFontSize {
		t.Errorf(
			"notified virtual pointer font size = %d, want the settled default %d",
			got,
			config.DefaultVirtualPointerFontSize,
		)
	}
}

// TestAdapterShowFrame_CarriesTheResolvedPointerFamily is the third and last
// way the pointer's family reaches a backend: on the recursive-grid frame.
// The Linux and Windows draws take that name straight to the text layer now
// that nothing resolves a second time downstream, so a frame carrying the
// written name would put an alias in front of a font engine again (#1305).
//
// Not parallel: the font resolver is process-wide.
func TestAdapterShowFrame_CarriesTheResolvedPointerFamily(t *testing.T) {
	ports.SetFontResolver(markerFontResolver{})
	t.Cleanup(func() { ports.SetFontResolver(nil) })

	cfg := config.DefaultConfig()
	cfg.VirtualPointer.UI.FontFamily = writtenAlias

	manager := newScreenManager()
	resolver := overlay.NewStyleResolver(manager, cfg, &countingTheme{}, zap.NewNop())
	adapter := overlay.NewAdapter(manager, resolver, zap.NewNop())

	err := adapter.ShowFrame(context.Background(), ports.RecursiveGridFrame{
		Bounds: image.Rect(10, 20, 210, 220),
		Layout: ports.RecursiveGridLayout{
			Keys:       "uiop",
			Dimensions: domain.GridDimensions{Rows: 2, Cols: 2},
		},
		NextLayout: ports.RecursiveGridLayout{
			Keys:       "hjkl",
			Dimensions: domain.GridDimensions{Rows: 2, Cols: 2},
		},
		Pointer: ports.GridPointer{Visible: true, Position: image.Pt(30, 40)},
	})
	if err != nil {
		t.Fatalf("ShowFrame() error = %v", err)
	}

	if got := manager.recursiveGrid.pointer.FontName; got != resolvedAlias {
		t.Errorf(
			"pointer font name on the frame = %q, want the resolved %q",
			got,
			resolvedAlias,
		)
	}
}

// TestStyleResolver_SubtitleFamilyFallsBackBeforeResolution pins that an unset
// subtitle family still means "draw the subtitle in the label's family". The
// resolver answers an empty name with the platform's sans-serif face, so
// resolving first and falling back afterwards would never fall back at all,
// and a monitor picker configured with one family would draw its subtitles in
// another.
//
// Not parallel: the font resolver is process-wide.
func TestStyleResolver_SubtitleFamilyFallsBackBeforeResolution(t *testing.T) {
	ports.SetFontResolver(markerFontResolver{})
	t.Cleanup(func() { ports.SetFontResolver(nil) })

	cfg := config.DefaultConfig()
	cfg.MonitorSelect.UI.FontFamily = "JetBrains Mono"
	cfg.MonitorSelect.UI.SubtitleFontFamily = ""

	style := overlay.NewStyleResolver(
		&headlessManager{}, cfg, &countingTheme{}, zap.NewNop(),
	).Style()

	if got := style.MonitorSelect.SubtitleFontFamily; got != style.MonitorSelect.FontFamily {
		t.Errorf(
			"subtitle font family = %q, want the label's %q",
			got,
			style.MonitorSelect.FontFamily,
		)
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
