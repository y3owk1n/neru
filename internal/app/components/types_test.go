package components_test

import (
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// testCharacters is the label alphabet the grid fixtures are built from.
const testCharacters = "abcd"

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

	manager := domainGrid.NewManager(
		initial,
		domain.GridDimensions{Rows: 2, Cols: 2},
		characters,
		nil,
		nil,
		log,
	)

	return &components.GridComponent{Manager: manager}
}

// gridConfig returns a default config with the grid enabled and the given
// labels applied, resolved the way the loader resolves one before any consumer
// sees it. UpdateConfig reads the labels rather than inferring them, so a
// fixture that skipped the resolution would be testing a config the daemon
// never runs on.
func gridConfig(characters, rowLabels, colLabels string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Grid.Enabled = true
	cfg.Grid.Characters = characters
	cfg.Grid.RowLabels = rowLabels
	cfg.Grid.ColLabels = colLabels
	cfg.ResolveGridLabels()

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
		// wantCharacters is the set the grid must end up describing, for the
		// cases where that is not just newCharacters upper-cased.
		wantCharacters string
		wantRecreate   bool
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
		{
			// A repeat is dropped when the grid is built, so a set that only
			// gained one is the same set. Comparing the string as written would
			// see a change on every reload and rebuild the grid each time.
			name:       "a character listed twice keeps the existing grid",
			characters: testCharacters, rowLabels: "", colLabels: "",
			newCharacters: "abcda", newRowLabels: "", newColLabels: "",
			wantCharacters: "ABCD",
			wantRecreate:   false,
		},
		{
			// The same for a label set, which the derivation settles with its
			// repeats still in it so the config can report them: the grid dropped
			// the repeat, so the two describe the same labels.
			name:       "a row label listed twice keeps the existing grid",
			characters: testCharacters, rowLabels: "ab", colLabels: "cd",
			newCharacters: testCharacters, newRowLabels: "aba", newColLabels: "cd",
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
			want := testCase.wantCharacters
			if want == "" {
				want = upper(testCase.newCharacters)
			}

			if got := after.Characters(); got != want {
				t.Errorf("Characters() = %q, want %q", got, want)
			}
		})
	}
}

// TestGridComponent_UpdateConfig_DefaultConfigReloadKeepsGrid is a regression
// test for a spurious rebuild on every `neru config reload`.
//
// A config file that leaves row_labels and col_labels empty means "infer from
// characters". The grid stores the *inferred* labels, so comparing them against
// the bare "" from config never matched and the grid was rebuilt on every
// reload — throwing away the user's in-flight coordinate input while grid mode
// was open. Since the option is resolved at load time, DefaultConfig() already
// carries the inferred labels and the comparison is a plain one.
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

// TestGridComponent_UpdateConfig_RecreatesGridOnMaxLabelLengthChange pins hot
// reload for the geometry option: the manager must start using the new coarse
// label bound immediately, and an unchanged follow-up reload must keep it.
func TestGridComponent_UpdateConfig_RecreatesGridOnMaxLabelLengthChange(t *testing.T) {
	component := newGridComponent(t, testCharacters, "", "")
	before := component.Manager.Grid()

	cfg := gridConfig(testCharacters, "", "")
	cfg.Grid.MaxLabelLength = 2
	component.UpdateConfig(cfg, zap.NewNop())

	after := component.Manager.Grid()
	if after == before {
		t.Fatal("changing max_label_length did not recreate the grid")
	}

	if got := after.MaxLabelLength(); got != 2 {
		t.Errorf("MaxLabelLength() = %d, want 2", got)
	}

	component.UpdateConfig(cfg, zap.NewNop())

	if component.Manager.Grid() != after {
		t.Error("reloading an unchanged max_label_length rebuilt the grid")
	}
}

// TestGridComponent_UpdateConfig_DisabledGridIsLeftAlone makes sure a disabled
// grid is not silently rebuilt behind the user's back.
func TestGridComponent_UpdateConfig_DisabledGridIsLeftAlone(t *testing.T) {
	component := newGridComponent(t, testCharacters, "", "")
	before := component.Manager.Grid()

	cfg := gridConfig("wxyz", "", "")
	cfg.Grid.Enabled = false

	component.UpdateConfig(cfg, zap.NewNop())

	if component.Manager.Grid() != before {
		t.Error("grid was recreated even though cfg.Grid.Enabled is false")
	}
}

// TestGridComponent_UpdateConfig_NilManagerAndGrid covers the guarded paths.
// Config reload runs on a live daemon, so a component that is not fully wired
// yet must be skipped rather than panicking and taking the daemon down.
func TestGridComponent_UpdateConfig_NilManagerAndGrid(t *testing.T) {
	t.Run("nil manager", func(t *testing.T) {
		component := &components.GridComponent{}

		component.UpdateConfig(gridConfig(testCharacters, "", ""), zap.NewNop())

		if component.Manager != nil {
			t.Error("UpdateConfig built a grid manager for a component that has none")
		}
	})

	t.Run("manager with nil grid", func(t *testing.T) {
		manager := domainGrid.NewManager(
			nil,
			domain.GridDimensions{Rows: 2, Cols: 2},
			testCharacters,
			nil,
			nil,
			zap.NewNop(),
		)
		component := &components.GridComponent{Manager: manager}

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
