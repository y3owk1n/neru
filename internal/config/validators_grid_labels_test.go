package config_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// validateGridWithWarnings runs the whole ladder and returns what it collected,
// failing the test if the configuration was refused. Every case here describes a
// file that loads: the point of the tier is that a label set the grid cannot use
// costs the user the label set, not the rest of their configuration (ADR 0002).
func validateGridWithWarnings(t *testing.T, cfg *config.Config) []string {
	t.Helper()

	warnings := &config.Warnings{}

	err := cfg.ValidateWithWarnings(warnings)
	if err != nil {
		t.Fatalf("ValidateWithWarnings() refused a configuration that loads: %v", err)
	}

	return warnings.Messages()
}

// warningsMentioning returns the collected warnings that name a field, so a case
// asserts about the field it changed rather than about the whole list.
func warningsMentioning(warnings []string, field string) []string {
	var found []string

	for _, warning := range warnings {
		if strings.Contains(warning, field) {
			found = append(found, warning)
		}
	}

	return found
}

// TestConfigValidateGrid_ResolvedLabelsAreSilent is the half that must not fire.
// The default labels are the ones inferred from grid.characters, and a user who
// wrote nothing must hear nothing — a warning naming a field that is not in
// their file sends them looking for a line they never wrote.
func TestConfigValidateGrid_ResolvedLabelsAreSilent(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg.Grid.RowLabels == "" || cfg.Grid.ColLabels == "" {
		t.Fatalf("DefaultConfig() left labels unresolved (row %q, col %q); "+
			"this test would then be checking the raw-empty case instead",
			cfg.Grid.RowLabels, cfg.Grid.ColLabels)
	}

	if got := validateGridWithWarnings(t, cfg); len(got) > 0 {
		t.Errorf("warnings = %q, want none", got)
	}
}

// TestConfigValidateGrid_RawEmptyLabelsAreSilent covers the other shape a
// configuration can be in: the value the user wrote, before the derivation
// settles it. An empty label set means "infer from characters", so it is
// nothing to warn about — checking the raw value rather than the resolved one
// would warn on every configuration that leaves the option alone.
func TestConfigValidateGrid_RawEmptyLabelsAreSilent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Grid.RowLabels = ""
	cfg.Grid.ColLabels = ""
	cfg.Grid.SublayerKeys = ""

	if got := validateGridWithWarnings(t, cfg); len(got) > 0 {
		t.Errorf("warnings = %q, want none", got)
	}
}

// TestConfigValidateGrid_ShortLabelsWarn pins the too-short case. One label
// cannot name more than one row, so a grid taller than a single cell has rows
// the user cannot reach.
func TestConfigValidateGrid_ShortLabelsWarn(t *testing.T) {
	tests := []struct {
		name  string
		set   func(*config.Config)
		field string
	}{
		{
			name:  "row labels",
			set:   func(c *config.Config) { c.Grid.RowLabels = "A" },
			field: "grid.row_labels",
		},
		{
			name:  "col labels",
			set:   func(c *config.Config) { c.Grid.ColLabels = "A" },
			field: "grid.col_labels",
		},
		{
			name:  "characters",
			set:   func(c *config.Config) { c.Grid.Characters = "a" },
			field: "grid.characters",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			testCase.set(cfg)

			got := warningsMentioning(validateGridWithWarnings(t, cfg), testCase.field)
			if len(got) != 1 {
				t.Fatalf("warnings for %s = %q, want exactly one", testCase.field, got)
			}

			if !strings.Contains(got[0], "at least") {
				t.Errorf("warning %q does not say how many characters are needed", got[0])
			}
		})
	}
}

// TestConfigValidateGrid_ShortSetsSayWhatTheyCost keeps the two too-short
// warnings from wearing each other's consequence. A short grid.characters is
// not used at all — the grid is relabelled a-z — while short labels are used as
// written and cap the grid instead, and a warning that swapped the two would
// send the user to fix the wrong thing.
func TestConfigValidateGrid_ShortSetsSayWhatTheyCost(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Grid.Characters = "a"

	got := warningsMentioning(validateGridWithWarnings(t, cfg), "grid.characters")
	if len(got) != 1 {
		t.Fatalf("warnings for grid.characters = %q, want exactly one", got)
	}

	if !strings.Contains(got[0], "a-z") {
		t.Errorf("warning %q does not say the grid is labeled a-z instead", got[0])
	}

	cfg = config.DefaultConfig()
	cfg.Grid.RowLabels = "a"

	got = warningsMentioning(validateGridWithWarnings(t, cfg), "grid.row_labels")
	if len(got) != 1 {
		t.Fatalf("warnings for grid.row_labels = %q, want exactly one", got)
	}

	if strings.Contains(got[0], "a-z") {
		t.Errorf("warning %q claims labels the user wrote are replaced, which they are not", got[0])
	}
}

// TestConfigValidateGrid_SublayerKeysAreNotCheckedHere pins the decision issue
// #1271 left open. The runtime trims, uppercases and caps sublayer_keys at the
// number of subgrid cells, so a fault past that cap is in a character that is
// never drawn — and this package cannot see the subgrid size to tell the two
// apart. Silence here is the choice, not an oversight.
func TestConfigValidateGrid_SublayerKeysAreNotCheckedHere(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Grid.SublayerKeys = "a a"

	got := warningsMentioning(validateGridWithWarnings(t, cfg), "grid.sublayer_keys")
	if len(got) > 0 {
		t.Errorf("warnings for grid.sublayer_keys = %q, want none", got)
	}
}

// TestConfigValidateGrid_DuplicateLabelsWarn pins the duplicate case. Matching
// folds case, so a repeat makes two rows answer to the same keystroke and only
// one of them can win.
func TestConfigValidateGrid_DuplicateLabelsWarn(t *testing.T) {
	tests := []struct {
		name  string
		set   func(*config.Config)
		field string
	}{
		{
			name:  "row labels",
			set:   func(c *config.Config) { c.Grid.RowLabels = "aba" },
			field: "grid.row_labels",
		},
		{
			name:  "col labels repeated case-insensitively",
			set:   func(c *config.Config) { c.Grid.ColLabels = "abA" },
			field: "grid.col_labels",
		},
		{
			name:  "characters",
			set:   func(c *config.Config) { c.Grid.Characters = "abb" },
			field: "grid.characters",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			testCase.set(cfg)

			got := warningsMentioning(validateGridWithWarnings(t, cfg), testCase.field)
			if len(got) != 1 {
				t.Fatalf("warnings for %s = %q, want exactly one", testCase.field, got)
			}
		})
	}
}

// TestConfigValidateGrid_RepeatedCharacterIsReportedOnce keeps the report
// proportional to the fault: a label set that repeats one character three times
// is one thing wrong, not two.
func TestConfigValidateGrid_RepeatedCharacterIsReportedOnce(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Grid.RowLabels = "aaab"

	got := warningsMentioning(validateGridWithWarnings(t, cfg), "grid.row_labels")
	if len(got) != 1 {
		t.Fatalf("warnings for grid.row_labels = %q, want exactly one", got)
	}
}

// TestConfigValidateGrid_RepeatsThatLeaveTooFewCharactersWarn covers the two
// faults that arrive as one. Repeats come out before the grid applies its floor
// (domain/grid.newGridAlphabet), so "aa" is a set of two characters that can
// label with one, and the grid replaces it with a-z: the repeat and the set being
// too short are both true, and counting the written length would have reported
// neither.
func TestConfigValidateGrid_RepeatsThatLeaveTooFewCharactersWarn(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Grid.Characters = "aa"

	got := warningsMentioning(validateGridWithWarnings(t, cfg), "grid.characters")

	want := 2
	if len(got) != want {
		t.Fatalf("warnings for grid.characters = %q, want %d", got, want)
	}

	for _, expected := range []string{"more than once", "usable character"} {
		if !strings.Contains(strings.Join(got, "\n"), expected) {
			t.Errorf("warnings = %q, want one mentioning %q", got, expected)
		}
	}
}

// TestConfigValidateGrid_UntypeableLabelsWarn covers the characters that load
// and cannot be pressed: whitespace, control characters and anything outside
// ASCII, none of which a user can type at a label prompt.
func TestConfigValidateGrid_UntypeableLabelsWarn(t *testing.T) {
	tests := []struct {
		name   string
		labels string
	}{
		{name: "space", labels: "a b"},
		{name: "tab", labels: "a\tb"},
		{name: "non-ascii", labels: "abé"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Grid.RowLabels = testCase.labels

			got := warningsMentioning(validateGridWithWarnings(t, cfg), "grid.row_labels")
			if len(got) != 1 {
				t.Fatalf("warnings for grid.row_labels = %q, want exactly one", got)
			}
		})
	}
}

// TestConfigValidateGrid_InferredLabelsReportTheirSource is what keeps one
// mistake from reading as four. Labels left empty are the characters, so a fault
// in grid.characters reaches row_labels, col_labels and sublayer_keys as well —
// and reporting it against all four would name three fields the user never
// wrote and cannot fix.
func TestConfigValidateGrid_InferredLabelsReportTheirSource(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Grid.Characters = "abb"
	cfg.Grid.RowLabels = ""
	cfg.Grid.ColLabels = ""
	cfg.Grid.SublayerKeys = ""
	cfg.ResolveDerived()

	got := validateGridWithWarnings(t, cfg)

	if len(got) != 1 {
		t.Fatalf("warnings = %q, want exactly one naming grid.characters", got)
	}

	if !strings.Contains(got[0], "grid.characters") {
		t.Errorf("warning = %q, want it to name grid.characters", got[0])
	}
}

// TestConfigValidateGrid_WrittenLabelsAreReportedThemselves is the other side of
// that rule: a label set the user did write is judged on its own, whatever
// grid.characters says.
func TestConfigValidateGrid_WrittenLabelsAreReportedThemselves(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Grid.RowLabels = "xyx"
	cfg.ResolveDerived()

	got := validateGridWithWarnings(t, cfg)

	want := 1
	if len(got) != want {
		t.Fatalf("warnings = %q, want %d naming grid.row_labels", got, want)
	}

	if !strings.Contains(got[0], "grid.row_labels") {
		t.Errorf("warning = %q, want it to name grid.row_labels", got[0])
	}
}

// TestConfigValidateGrid_DisabledGridIsSilent keeps the warnings consistent with
// the refusals around them: ValidateGrid returns early for a disabled grid so a
// user can leave a section they do not use alone, and a warning that fired
// anyway would be advice about a mode that never runs.
func TestConfigValidateGrid_DisabledGridIsSilent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Grid.Enabled = false
	cfg.Grid.RowLabels = "A"

	if got := validateGridWithWarnings(t, cfg); len(got) > 0 {
		t.Errorf("warnings = %q, want none for a disabled grid", got)
	}
}
