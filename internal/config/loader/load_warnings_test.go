package loader_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
)

// accelInertWarning is what a configuration that turns acceleration on without
// the repeat it would scale is told.
const accelInertWarning = "held_repeat.accel_enabled has no effect while held_repeat.enabled is false"

// TestLoadWithValidation_ReportsAnInertAccelSetting pins the whole path the
// warnings tier exists for: the file loads, the setting stays, and the reason
// it will do nothing rides out on the result so `neru config validate` can
// print it (ADR 0002, ADR 0006).
func TestLoadWithValidation_ReportsAnInertAccelSetting(t *testing.T) {
	result, logs := loadWithObservedLogger(t, `
[held_repeat]
enabled = false
accel_enabled = true
`, "")

	if result.ValidationError != nil {
		t.Fatalf("an inert setting was refused: %v", result.ValidationError)
	}

	if !slices.Contains(result.Warnings, accelInertWarning) {
		t.Errorf("result.Warnings = %q, want it to contain %q", result.Warnings, accelInertWarning)
	}

	// The setting the user wrote survives: a warning reports, it does not undo.
	if !result.Config.HeldRepeat.AccelEnabled {
		t.Error("held_repeat.accel_enabled was cleared, want it left as written")
	}

	// Said once. The daemon logs everything on the result, so a check that also
	// logs on its own would say it twice to whoever reads the log.
	if got := countLogged(logs, accelInertWarning); got != 1 {
		t.Errorf("the warning was logged %d times, want exactly 1", got)
	}
}

// TestLoadWithValidation_ReadsAnInertAccelSettingAfterOverrides pins that the
// reading is taken from the configuration the daemon will run on. `neru config
// set held_repeat.enabled false` writes an override, and that alone is enough
// to make an accel_enabled the user wrote months ago inert.
func TestLoadWithValidation_ReadsAnInertAccelSettingAfterOverrides(t *testing.T) {
	result, logs := loadWithObservedLogger(t, `
[held_repeat]
enabled = true
accel_enabled = true
`, `
[held_repeat]
enabled = false
`)

	if result.ValidationError != nil {
		t.Fatalf("an inert setting was refused: %v", result.ValidationError)
	}

	if !slices.Contains(result.Warnings, accelInertWarning) {
		t.Errorf("result.Warnings = %q, want it to contain %q", result.Warnings, accelInertWarning)
	}

	if got := countLogged(logs, accelInertWarning); got != 1 {
		t.Errorf("the warning was logged %d times, want exactly 1", got)
	}
}

// TestLoadWithValidation_AnOverrideAnswersAnInertAccelSetting is why the second
// reading replaces the first rather than adding to it. Turning held-key repeat
// back on is the answer to the warning, so a merge would leave it standing and
// `neru config validate` would report a problem the user had just fixed.
func TestLoadWithValidation_AnOverrideAnswersAnInertAccelSetting(t *testing.T) {
	result, logs := loadWithObservedLogger(t, `
[held_repeat]
enabled = false
accel_enabled = true
`, `
[held_repeat]
enabled = true
`)

	if result.ValidationError != nil {
		t.Fatalf("the config was refused: %v", result.ValidationError)
	}

	if slices.Contains(result.Warnings, accelInertWarning) {
		t.Errorf("result.Warnings = %q, want the override to have answered it", result.Warnings)
	}

	if got := countLogged(logs, accelInertWarning); got != 0 {
		t.Errorf("the answered warning was logged %d times, want none", got)
	}
}

// TestLoadWithValidation_RereadsWarningsAfterAnOverride pins the other side of
// that replacement: an override answers only what it touches, so a warning the
// config file earned must be read again and still be there.
func TestLoadWithValidation_RereadsWarningsAfterAnOverride(t *testing.T) {
	result, _ := loadWithObservedLogger(t, `
[hotkeys]
"Primary+Shift+J" = "hints --repeat"
`, `
[hints.ui]
font_size = 30
`)

	if result.ValidationError != nil {
		t.Fatalf("the config was refused: %v", result.ValidationError)
	}

	want := "hotkeys.Primary+Shift+J: --repeat requires --action"
	if !slices.Contains(result.Warnings, want) {
		t.Errorf("result.Warnings = %q, want it to contain %q", result.Warnings, want)
	}
}

// TestLoadWithValidation_ReportsAHandWrittenLabelThatMatchesTheInference is
// issue #1281 through the path it was reported on. The space cannot be typed at
// a grid and is in both fields, so both are named in one load — the load is
// what keeps the pre-derivation configuration (config.LoadResult.Written),
// which is the only place a label the user typed can still be told from one the
// derivation settled. Judged by value alone the two are the same string, and
// the label's warning waited for the user to fix grid.characters and run the
// command a second time.
func TestLoadWithValidation_ReportsAHandWrittenLabelThatMatchesTheInference(t *testing.T) {
	result, _ := loadWithObservedLogger(t, `
[grid]
characters = "ab c"
row_labels = "AB C"
`, "")

	if result.ValidationError != nil {
		t.Fatalf("an untypeable label was refused: %v", result.ValidationError)
	}

	for _, field := range []string{"grid.characters", "grid.row_labels"} {
		if !slices.ContainsFunc(result.Warnings, func(w string) bool {
			return strings.Contains(w, field) && strings.Contains(w, "cannot be typed")
		}) {
			t.Errorf("result.Warnings = %q, want the space reported against %s",
				result.Warnings, field)
		}
	}
}

// TestLoadWithValidation_LeavesAnInferredLabelToItsSource is the other half,
// and the reason the answer above had to be the written configuration rather
// than every field the fault reached. The labels are left empty here, so they
// are grid.characters — reporting them too would send the user looking for two
// lines that are not in their file.
func TestLoadWithValidation_LeavesAnInferredLabelToItsSource(t *testing.T) {
	result, _ := loadWithObservedLogger(t, `
[grid]
characters = "ab c"
`, "")

	if result.ValidationError != nil {
		t.Fatalf("an untypeable character was refused: %v", result.ValidationError)
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("result.Warnings = %q, want exactly one naming grid.characters",
			result.Warnings)
	}

	if !strings.Contains(result.Warnings[0], "grid.characters") {
		t.Errorf("result.Warnings = %q, want it to name grid.characters", result.Warnings)
	}
}

// loadWithObservedLogger writes a config file, and an override file when one is
// given, then loads them the way the daemon does with a logger that records.
// The configure hooks stand in for what a composition root wires onto the
// service before the load.
func loadWithObservedLogger(
	t *testing.T,
	configTOML, overrideTOML string,
	configure ...func(*loader.Service) *loader.Service,
) (*config.LoadResult, *observer.ObservedLogs) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	writeErr := os.WriteFile(path, []byte(configTOML), 0o600)
	if writeErr != nil {
		t.Fatalf("WriteFile: %v", writeErr)
	}

	if overrideTOML != "" {
		overrideErr := os.WriteFile(
			filepath.Join(dir, "config.override.toml"),
			[]byte(overrideTOML),
			0o600,
		)
		if overrideErr != nil {
			t.Fatalf("WriteFile override: %v", overrideErr)
		}
	}

	core, logs := observer.New(zap.WarnLevel)

	svc := loader.NewService(nil, path, zap.New(core), nil)
	for _, hook := range configure {
		svc = hook(svc)
	}

	result := svc.LoadWithValidation(path)
	if result == nil || result.Config == nil {
		t.Fatalf("LoadWithValidation returned no config")
	}

	return result, logs
}

// countLogged reports how many entries mention text, in the message or in any
// of its fields, so a check that logs on its own is not hidden by wording.
func countLogged(logs *observer.ObservedLogs, text string) int {
	count := 0

	for _, entry := range logs.All() {
		if strings.Contains(entry.Message, text) {
			count++

			continue
		}

		for _, field := range entry.Context {
			if strings.Contains(field.String, text) {
				count++

				break
			}
		}
	}

	return count
}
