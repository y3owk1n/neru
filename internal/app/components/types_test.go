package components_test

import (
	"image"
	"reflect"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// testCharacters is the label alphabet the grid fixtures are built from.
const testCharacters = "abcd"

// lightTheme is a fixed config.ThemeProvider so built styles depend only on the
// config values a test varies.
type lightTheme struct{}

func (lightTheme) IsDarkMode() bool { return false }

// newGridComponent builds a GridComponent whose Manager holds a grid created
// from the given labels. Overlay is deliberately nil: the overlay is a native
// type, and the logic worth pinning here — when the grid gets recreated — lives
// entirely on the Manager side and is guarded by its own nil check.
func newGridComponent(
	t *testing.T,
	characters, rowLabels, colLabels string,
) *components.GridComponent {
	t.Helper()

	log := zap.NewNop()

	initial := domainGrid.NewGridWithLabels(
		characters,
		rowLabels,
		colLabels,
		image.Rect(0, 0, 800, 600),
		log,
	)

	manager := domainGrid.NewManager(initial, 2, 2, characters, nil, nil, log)

	return &components.GridComponent{
		Manager: manager,
		Theme:   lightTheme{},
	}
}

// gridConfig returns a default config with the grid enabled and the given
// labels applied.
func gridConfig(characters, rowLabels, colLabels string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Grid.Enabled = true
	cfg.Grid.Characters = characters
	cfg.Grid.RowLabels = rowLabels
	cfg.Grid.ColLabels = colLabels

	return cfg
}

// TestGridComponent_UpdateConfig_RecreatesGridOnLabelChanges pins exactly when a
// config reload rebuilds the grid. Rebuilding drops the user's in-flight
// coordinate input, so doing it too eagerly is as much a bug as not doing it at
// all — in particular, the comparison is case-insensitive, because the grid
// stores its labels upper-cased while config keeps whatever the user typed.
func TestGridComponent_UpdateConfig_RecreatesGridOnLabelChanges(t *testing.T) {
	tests := []struct {
		name string
		// initial labels the component starts with.
		characters, rowLabels, colLabels string
		// newCharacters etc. are what the reloaded config carries.
		newCharacters, newRowLabels, newColLabels string
		wantRecreate                              bool
	}{
		{
			name:       "identical config keeps the existing grid",
			characters: testCharacters, rowLabels: "", colLabels: "",
			newCharacters: testCharacters, newRowLabels: "", newColLabels: "",
			wantRecreate: false,
		},
		{
			name:       "characters changing case only keeps the existing grid",
			characters: testCharacters, rowLabels: "", colLabels: "",
			newCharacters: "ABCD", newRowLabels: "", newColLabels: "",
			wantRecreate: false,
		},
		{
			name:       "different characters recreate the grid",
			characters: testCharacters, rowLabels: "", colLabels: "",
			newCharacters: "wxyz", newRowLabels: "", newColLabels: "",
			wantRecreate: true,
		},
		{
			name:       "different row labels recreate the grid",
			characters: testCharacters, rowLabels: "ab", colLabels: "cd",
			newCharacters: testCharacters, newRowLabels: "cd", newColLabels: "cd",
			wantRecreate: true,
		},
		{
			name:       "different column labels recreate the grid",
			characters: testCharacters, rowLabels: "ab", colLabels: "cd",
			newCharacters: testCharacters, newRowLabels: "ab", newColLabels: "ab",
			wantRecreate: true,
		},
		{
			name:       "row labels changing case only keeps the existing grid",
			characters: testCharacters, rowLabels: "ab", colLabels: "cd",
			newCharacters: testCharacters, newRowLabels: "AB", newColLabels: "cd",
			wantRecreate: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			component := newGridComponent(
				t,
				testCase.characters,
				testCase.rowLabels,
				testCase.colLabels,
			)

			before := component.Manager.Grid()

			component.UpdateConfig(
				gridConfig(testCase.newCharacters, testCase.newRowLabels, testCase.newColLabels),
				zap.NewNop(),
			)

			after := component.Manager.Grid()

			if after == nil {
				t.Fatal("Manager.Grid() is nil after UpdateConfig")
			}

			if recreated := after != before; recreated != testCase.wantRecreate {
				t.Errorf("grid recreated = %t, want %t", recreated, testCase.wantRecreate)
			}

			// Whether or not it was rebuilt, the grid must end up describing
			// the labels the reloaded config asked for.
			if got, want := after.Characters(), upper(testCase.newCharacters); got != want {
				t.Errorf("Characters() = %q, want %q", got, want)
			}
		})
	}
}

// TestGridComponent_UpdateConfig_DefaultConfigReloadKeepsGrid is a regression
// test for a spurious rebuild on every `neru config reload`.
//
// The default config leaves RowLabels and ColLabels empty, meaning "infer from
// characters". The grid stores the *inferred* labels, so comparing them against
// the bare "" from config never matched and the grid was rebuilt on every
// reload — throwing away the user's in-flight coordinate input while grid mode
// was open. The comparison now resolves empty labels the same way the grid
// constructor does.
func TestGridComponent_UpdateConfig_DefaultConfigReloadKeepsGrid(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Grid.Enabled = true

	component := newGridComponent(t, cfg.Grid.Characters, cfg.Grid.RowLabels, cfg.Grid.ColLabels)
	before := component.Manager.Grid()

	// Two reloads of the very same config, as a user editing an unrelated
	// setting would produce.
	component.UpdateConfig(cfg, zap.NewNop())
	component.UpdateConfig(cfg, zap.NewNop())

	if component.Manager.Grid() != before {
		t.Error("reloading an unchanged default config rebuilt the grid, discarding grid input")
	}
}

// TestGridComponent_UpdateConfig_RecreatedGridKeepsBounds checks that a rebuild
// reuses the old grid's bounds. Falling back to a zero rectangle here would
// place every cell off-screen after a config reload.
func TestGridComponent_UpdateConfig_RecreatedGridKeepsBounds(t *testing.T) {
	component := newGridComponent(t, testCharacters, "", "")
	wantBounds := component.Manager.Grid().Bounds()

	component.UpdateConfig(gridConfig("wxyz", "", ""), zap.NewNop())

	if got := component.Manager.Grid().Bounds(); got != wantBounds {
		t.Errorf("recreated grid bounds = %v, want %v", got, wantBounds)
	}
}

// TestGridComponent_UpdateConfig_DisabledGridIsLeftAlone makes sure a disabled
// grid is not silently rebuilt or restyled behind the user's back.
func TestGridComponent_UpdateConfig_DisabledGridIsLeftAlone(t *testing.T) {
	component := newGridComponent(t, testCharacters, "", "")
	before := component.Manager.Grid()
	beforeStyle := component.Style

	cfg := gridConfig("wxyz", "", "")
	cfg.Grid.Enabled = false

	component.UpdateConfig(cfg, zap.NewNop())

	if component.Manager.Grid() != before {
		t.Error("grid was recreated even though cfg.Grid.Enabled is false")
	}

	if component.Style != beforeStyle {
		t.Error("style was rebuilt even though cfg.Grid.Enabled is false")
	}
}

// TestGridComponent_UpdateConfig_BuildsStyle checks the style is refreshed from
// the new config, since that is the only path by which a theme or font change
// reaches the grid overlay.
func TestGridComponent_UpdateConfig_BuildsStyle(t *testing.T) {
	component := newGridComponent(t, testCharacters, "", "")

	cfg := gridConfig(testCharacters, "", "")
	cfg.Grid.UI.FontSize = 33
	cfg.Grid.UI.BorderWidth = 7

	want := grid.BuildStyle(cfg.Grid, lightTheme{})

	component.UpdateConfig(cfg, zap.NewNop())

	if component.Style != want {
		t.Error("Style was not rebuilt from the reloaded config")
	}

	// Guard against the assertion being vacuous: the style built from the
	// reloaded config must differ from the zero value it started at.
	if want == (grid.Style{}) {
		t.Fatal("BuildStyle produced the zero Style; the assertion above proves nothing")
	}
}

// TestGridComponent_UpdateConfig_NilManagerAndGrid covers the guarded paths.
// Config reload runs on a live daemon, so a component that is not fully wired
// yet must be skipped rather than panicking and taking the daemon down.
func TestGridComponent_UpdateConfig_NilManagerAndGrid(t *testing.T) {
	t.Run("nil manager", func(t *testing.T) {
		component := &components.GridComponent{Theme: lightTheme{}}

		component.UpdateConfig(gridConfig(testCharacters, "", ""), zap.NewNop())

		if component.Style == (grid.Style{}) {
			t.Error("Style was not built when Manager is nil")
		}
	})

	t.Run("manager with nil grid", func(t *testing.T) {
		manager := domainGrid.NewManager(nil, 2, 2, testCharacters, nil, nil, zap.NewNop())
		component := &components.GridComponent{Manager: manager, Theme: lightTheme{}}

		component.UpdateConfig(gridConfig("wxyz", "", ""), zap.NewNop())

		if component.Manager.Grid() != nil {
			t.Error("a nil grid was replaced; UpdateConfig should only recreate an existing grid")
		}
	})

	t.Run("empty characters in config", func(t *testing.T) {
		component := newGridComponent(t, testCharacters, "", "")
		before := component.Manager.Grid()

		cfg := gridConfig("", "", "")

		component.UpdateConfig(cfg, zap.NewNop())

		if component.Manager.Grid() != before {
			t.Error("grid was recreated from an empty character set")
		}
	})
}

// TestHintsComponent_UpdateConfig_OnlyAppliesWhenEnabled pins the enabled gate.
// HintsComponent.UpdateConfig also requires a non-nil overlay, so a component
// without one must leave its style untouched even when hints are enabled.
func TestHintsComponent_UpdateConfig_OnlyAppliesWhenEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true
	cfg.Hints.UI.FontSize = 29

	component := &components.HintsComponent{Theme: lightTheme{}}
	before := component.Style

	component.UpdateConfig(cfg, zap.NewNop())

	// Overlay is nil, so nothing should have been applied.
	if component.Style != before {
		t.Error("Style changed even though Overlay is nil")
	}

	if before != (hints.StyleMode{}) {
		t.Fatal("expected the initial style to be the zero value")
	}
}

// TestScrollComponent_UpdateConfig_IsANoOp documents that ScrollComponent
// intentionally carries no config-derived state. If it ever gains some, this
// test should be replaced rather than deleted.
func TestScrollComponent_UpdateConfig_IsANoOp(t *testing.T) {
	component := &components.ScrollComponent{}

	component.UpdateConfig(config.DefaultConfig(), zap.NewNop())

	if component.Context != nil {
		t.Errorf("UpdateConfig populated Context = %v, want it left nil", component.Context)
	}
}

// TestComponents_UpdateConfig_NilOverlayLeavesComponentUntouched covers the
// overlay-backed components. Their overlays are native types that cannot be
// constructed in a unit test, so what is checkable — and what actually matters
// during a live config reload — is that a component with no overlay wired up is
// skipped entirely: not dereferenced (which would panic and take the daemon
// down), and not partially mutated into a state inconsistent with its overlay.
func TestComponents_UpdateConfig_NilOverlayLeavesComponentUntouched(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RecursiveGrid.Enabled = true
	cfg.Hints.Enabled = true
	cfg.Hints.UI.FontSize = 31
	cfg.ModeIndicator.UI.FontSize = 31

	log := zap.NewNop()

	tests := []struct {
		name      string
		component interface {
			UpdateConfig(cfg *config.Config, logger *zap.Logger)
		}
	}{
		{"hints", &components.HintsComponent{Theme: lightTheme{}}},
		{"modeIndicator", &components.ModeIndicatorComponent{}},
		{"stickyIndicator", &components.StickyIndicatorComponent{}},
		{"recursiveGrid", &components.RecursiveGridComponent{Theme: lightTheme{}}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// Snapshot the component before the reload. Every field is a
			// pointer, interface or comparable style value, so a deep copy of
			// the pointed-to struct is a faithful "before" image.
			before := reflect.ValueOf(testCase.component).Elem().Interface()

			testCase.component.UpdateConfig(cfg, log)

			after := reflect.ValueOf(testCase.component).Elem().Interface()

			if !reflect.DeepEqual(before, after) {
				t.Errorf(
					"UpdateConfig mutated a component with no overlay:\nbefore: %+v\nafter:  %+v",
					before, after,
				)
			}
		})
	}
}

// upper mirrors the upper-casing the grid applies to its labels.
func upper(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'a' && r <= 'z' {
			out[i] = r - 32
		}
	}

	return string(out)
}
