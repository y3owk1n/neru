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

// loadWithObservedLogger writes a config file, and an override file when one is
// given, then loads them the way the daemon does with a logger that records.
func loadWithObservedLogger(
	t *testing.T,
	configTOML, overrideTOML string,
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

	result := loader.NewService(nil, path, zap.New(core), nil).LoadWithValidation(path)
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
