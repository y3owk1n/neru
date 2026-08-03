package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

// LoadWithValidation turns a config file into the Config the whole daemon runs
// on. Everything it does — merging hotkeys over defaults, honoring the disable
// sentinel, reading per-mode sections, layering the override file — is only
// visible in the Config that comes out.
//
// These cases load a representative file and compare the whole resulting Config
// against a snapshot taken from the same input, so a change anywhere in the load
// path shows up as a diff rather than as a silently different daemon.

// loadCase is one config file and the aspects of the load worth naming.
type loadCase struct {
	name     string
	config   string
	override string
}

// loadSnapshot is what a case records: the config the daemon would run on, and
// whether the load refused it. A config that fails validation still comes back
// populated, so recording only the config would let a refusal read as a clean
// load.
type loadSnapshot struct {
	Config          *Config `json:"config"`
	ValidationError string  `json:"validationError"`
}

func loadCases() []loadCase {
	return []loadCase{
		{
			name:   "empty file falls back to defaults",
			config: "",
		},
		{
			name: "user hotkeys merge over the defaults",
			config: `
[hotkeys]
"Primary+Shift+K" = "hints"
`,
		},
		{
			name: "a hotkey naming a platform modifier is refused",
			config: `
[hotkeys]
"cmd+shift+k" = "hints"
`,
		},
		{
			name: "the disable sentinel removes a default binding",
			config: `
[hotkeys]
"Primary+Shift+Space" = "__disabled__"
`,
		},
		{
			name: "an empty hotkeys section clears every binding",
			config: `
[hotkeys]
`,
		},
		{
			// Two actions, so that the rule replacing a rebound built-in
			// launcher does not fire and the case is left testing only the
			// case-folding it is named for.
			name: "a differently cased hotkey replaces the default it matches",
			config: `
[hotkeys]
"Primary+Shift+g" = ["grid", "hints"]
`,
		},
		{
			name: "a hotkeys section may carry an app_configs table",
			config: `
[hotkeys]
"Primary+Shift+P" = "hints"
[[hotkeys.app_configs]]
bundle_id = "com.apple.Safari"
`,
		},
		{
			name: "per-mode hotkeys merge over the mode defaults",
			config: `
[hints.hotkeys]
"j" = "scroll_down"
`,
		},
		{
			name: "a differently cased mode hotkey replaces its default",
			config: `
[hints.hotkeys]
"escape" = "exit"
`,
		},
		{
			name: "an empty mode hotkeys section clears that mode's bindings",
			config: `
[hints.hotkeys]
`,
		},
		{
			name: "a mode section overrides scalar settings",
			config: `
[hints.ui]
font_size = 42
`,
		},
		{
			name: "an app config carries its own hints settings",
			config: `
[[app_configs]]
bundle_id = "com.apple.Safari"
[app_configs.hints.ui]
font_size = 21
`,
		},
		{
			name: "the override file wins over the config file",
			config: `
[hints.ui]
font_size = 10
`,
			override: `
[hints.ui]
font_size = 30
`,
		},
	}
}

// loadResultFor writes a case to disk and loads it the way the daemon does.
func loadResultFor(t *testing.T, testCase loadCase) loadSnapshot {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	writeErr := os.WriteFile(path, []byte(testCase.config), 0o600)
	if writeErr != nil {
		t.Fatalf("WriteFile: %v", writeErr)
	}

	if testCase.override != "" {
		overrideErr := os.WriteFile(
			filepath.Join(dir, "config.override.toml"),
			[]byte(testCase.override),
			0o600,
		)
		if overrideErr != nil {
			t.Fatalf("WriteFile override: %v", overrideErr)
		}
	}

	service := NewService(DefaultConfig(), path, zap.NewNop(), nil)

	result := service.LoadWithValidation(path)
	if result == nil || result.Config == nil {
		t.Fatalf("LoadWithValidation returned no config")
	}

	snapshot := loadSnapshot{Config: result.Config}
	if result.ValidationError != nil {
		snapshot.ValidationError = result.ValidationError.Error()
	}

	return snapshot
}

// TestLoadWithValidationMatchesItsSnapshot compares the whole loaded Config
// against a recorded one. Set UPDATE_GOLDEN=1 to re-record after an intentional
// change, and read the diff carefully when it fails: it is the difference
// between two daemons.
func TestLoadWithValidationMatchesItsSnapshot(t *testing.T) {
	for _, testCase := range loadCases() {
		t.Run(testCase.name, func(t *testing.T) {
			loaded := loadResultFor(t, testCase)

			encoded, marshalErr := json.MarshalIndent(loaded, "", "  ")
			if marshalErr != nil {
				t.Fatalf("MarshalIndent: %v", marshalErr)
			}

			golden := filepath.Join("testdata", "load", testCase.name+".json")

			if os.Getenv("UPDATE_GOLDEN") != "" {
				mkdirErr := os.MkdirAll(filepath.Dir(golden), 0o750)
				if mkdirErr != nil {
					t.Fatalf("MkdirAll: %v", mkdirErr)
				}

				writeErr := os.WriteFile(golden, encoded, 0o600)
				if writeErr != nil {
					t.Fatalf("WriteFile golden: %v", writeErr)
				}

				return
			}

			want, readErr := os.ReadFile(golden)
			if readErr != nil {
				t.Fatalf("ReadFile golden (run with UPDATE_GOLDEN=1 to record): %v", readErr)
			}

			if string(encoded) != string(want) {
				t.Errorf("loaded config differs from %s", golden)
			}
		})
	}
}
