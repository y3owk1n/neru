package loader_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
)

// loadCase is one config file and the aspects of the load worth naming.
type loadCase struct {
	name     string
	config   string
	override string
}

// loadSnapshot is what a case records.
type loadSnapshot struct {
	// Delta is every path at which the loaded config differs from the baseline,
	// which is this same setup loading an empty file. It is left out when the
	// load was refused: a refusal hands back the platform defaults, and which
	// defaults those are is a platform question covered by other tests.
	Delta []string `json:"delta"`

	// ValidationError is the reason the file was refused, empty when it loaded.
	// A refused config still comes back populated, so recording only the config
	// would let a refusal read as a clean load.
	ValidationError string `json:"validationError"`

	// Warnings are the parts of the file that loaded and will not do what they
	// say. Recorded alongside the delta so a case can show both at once: that
	// the binding is there, and that it was reported.
	Warnings []string `json:"warnings,omitempty"`
}

// fixedHotkeyDefaults is the binding set every case starts from.
//
// The platform defaults disagree — Linux clears them entirely — so a case that
// merges over a default binding, replaces one, or disables one would be testing
// something different on each OS, and on Linux would be testing nothing at all.
// Injecting one set through the service's defaults gives every platform the same
// starting point.
func fixedHotkeyDefaults() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Hotkeys.Bindings = map[string][]string{
		"Primary+Shift+Space": {config.ModeNameHints},
		"Primary+Shift+G":     {config.ModeNameGrid},
		"Primary+Shift+C":     {config.ModeNameRecursiveGrid},
		"Primary+Shift+S":     {config.ModeNameScroll},
	}

	return cfg
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
"j" = "action move_mouse_relative --dx=0 --dy=10"
`,
		},
		{
			// The default is spelled "Escape"; this must replace it rather than
			// add a second binding that never fires.
			name: "a differently cased mode hotkey replaces its default",
			config: `
[hints.hotkeys]
"escape" = "idle"
`,
		},
		{
			name: "an empty mode hotkeys section clears that modes bindings",
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
			// The binding works minus the flag, so it keeps working: the delta
			// shows it loaded, and the warning says what it will not do.
			// Refusing it would cost the user every other binding in the file.
			name: "a binding with an inert flag loads and warns",
			config: `
[hotkeys]
"Primary+Shift+K" = "grid --search"
"Primary+Shift+J" = "hints --repeat"
`,
		},
		{
			// A flag nothing knows was never going to activate anything, so
			// this is refused like the unknown command it resembles.
			name: "a binding with an unknown flag is refused",
			config: `
[hotkeys]
"Primary+Shift+K" = "hints --serach --action=bogus"
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

// loadConfigFor writes a case to disk and loads it the way the daemon does.
func loadConfigFor(t *testing.T, testCase loadCase) *config.LoadResult {
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

	// WithDefaults is the injection point for the hotkey defaults; NewService's
	// first argument is the active config, not the defaults a load starts from.
	service := loader.NewService(nil, path, zap.NewNop(), nil).
		WithDefaults(fixedHotkeyDefaults())

	result := service.LoadWithValidation(path)
	if result == nil || result.Config == nil {
		t.Fatalf("LoadWithValidation returned no config")
	}

	return result
}

// flattenConfig renders a config as one entry per leaf, keyed by its path, so
// two configs can be compared without knowing their shape.
func flattenConfig(t *testing.T, cfg *config.Config) map[string]string {
	t.Helper()

	encoded, marshalErr := json.Marshal(cfg)
	if marshalErr != nil {
		t.Fatalf("Marshal: %v", marshalErr)
	}

	var tree any

	unmarshalErr := json.Unmarshal(encoded, &tree)
	if unmarshalErr != nil {
		t.Fatalf("Unmarshal: %v", unmarshalErr)
	}

	flat := make(map[string]string)
	flattenInto("", tree, flat)

	return flat
}

// flattenInto walks a decoded config, recording each leaf under its path.
func flattenInto(prefix string, value any, out map[string]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			flattenInto(prefix+"."+key, child, out)
		}
	case []any:
		for index, child := range typed {
			flattenInto(fmt.Sprintf("%s[%d]", prefix, index), child, out)
		}
	default:
		// A leaf decoded from JSON is a string, number, bool or nil, all of
		// which re-encode without error.
		encoded, encodeErr := json.Marshal(value)
		if encodeErr != nil {
			out[prefix] = "<unencodable>"

			return
		}

		out[prefix] = string(encoded)
	}
}

// configDelta lists every path at which loaded differs from baseline.
func configDelta(t *testing.T, baseline, loaded *config.Config) []string {
	t.Helper()

	before := flattenConfig(t, baseline)
	after := flattenConfig(t, loaded)

	lines := []string{}

	for path, value := range after {
		previous, existed := before[path]

		switch {
		case !existed:
			lines = append(lines, "+ "+path+" = "+value)
		case previous != value:
			lines = append(lines, "~ "+path+": "+previous+" -> "+value)
		}
	}

	for path, value := range before {
		_, exists := after[path]
		if !exists {
			lines = append(lines, "- "+path+" = "+value)
		}
	}

	sort.Strings(lines)

	return lines
}

// TestLoadWithValidation_MatchesItsSnapshot compares what each config file
// *changed* against a recorded delta. Whole-config snapshots cannot work: the
// defaults vary by platform and machine (Linux ships no global hotkeys, macOS
// adds menu-bar targets, Windows derives its shell from %SystemRoot%), while
// the change a file makes is the same everywhere. Set UPDATE_GOLDEN=1 to
// re-record after an intentional change; a failing diff is the difference
// between two daemons.
func TestLoadWithValidation_MatchesItsSnapshot(t *testing.T) {
	baseline := loadConfigFor(t, loadCase{name: "baseline"})
	if baseline.ValidationError != nil {
		t.Fatalf("the baseline config was refused: %v", baseline.ValidationError)
	}

	for _, testCase := range loadCases() {
		t.Run(testCase.name, func(t *testing.T) {
			result := loadConfigFor(t, testCase)

			snapshot := loadSnapshot{Delta: []string{}}
			if result.ValidationError != nil {
				snapshot.ValidationError = result.ValidationError.Error()
			} else {
				snapshot.Delta = configDelta(t, baseline.Config, result.Config)
				snapshot.Warnings = result.Warnings
			}

			encoded, marshalErr := json.MarshalIndent(snapshot, "", "  ")
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
				t.Errorf("loaded config differs from %s\n got: %s\nwant: %s",
					golden,
					strings.TrimSpace(string(encoded)),
					strings.TrimSpace(string(want)))
			}
		})
	}
}
