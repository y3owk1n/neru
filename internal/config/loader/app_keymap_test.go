package loader_test

import (
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// TestLoadWithValidation_AppHotkeysReachTheResolvedKeymap follows a per-app
// hotkey table from the file to the keymap a focused app gets. The load
// snapshots show the table was read. This shows the resolver applies it, so a
// break between loading and resolving fails here.
func TestLoadWithValidation_AppHotkeysReachTheResolvedKeymap(t *testing.T) {
	const safari = "com.apple.Safari"

	tests := []struct {
		name      string
		mode      string
		config    string
		bound     string
		wantSteps []string
		disabled  string
	}{
		{
			name: "hints",
			mode: config.ModeNameHints,
			config: `
[[hints.app_configs]]
bundle_id = "com.apple.Safari"
[hints.app_configs.hotkeys]
"Return" = ["action left_click", "hints"]
"Shift+L" = "__disabled__"
`,
			bound:     "Return",
			wantSteps: []string{"action left_click", config.ModeNameHints},
			disabled:  "Shift+L",
		},
		{
			name: "scroll",
			mode: config.ModeNameScroll,
			config: `
[[scroll.app_configs]]
bundle_id = "com.apple.Safari"
[scroll.app_configs.hotkeys]
"Return" = ["action scroll_down"]
"k" = "__disabled__"
`,
			bound:     "Return",
			wantSteps: []string{"action scroll_down"},
			disabled:  "k",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := loadConfigFor(t, loadCase{name: testCase.name, config: testCase.config})
			if result.ValidationError != nil {
				t.Fatalf("LoadWithValidation() error = %v", result.ValidationError)
			}

			disabledKey := config.NormalizeKeyForComparison(testCase.disabled)

			// Without this the disabled check below would also pass for a key
			// the mode never bound.
			base := result.Config.ResolveKeymap(testCase.mode, "")
			if _, inherited := base.Lookup(disabledKey); !inherited {
				t.Fatalf("base %s keymap does not bind %q", testCase.mode, testCase.disabled)
			}

			got := result.Config.ResolveKeymap(testCase.mode, safari)

			binding, bound := got.Lookup(config.NormalizeKeyForComparison(testCase.bound))
			if !bound || !slices.Equal(binding.Steps, testCase.wantSteps) {
				t.Errorf(
					"%q steps = %v, want %v",
					testCase.bound,
					binding.Steps,
					testCase.wantSteps,
				)
			}

			if _, exists := got.Lookup(disabledKey); exists {
				t.Errorf("%q is still bound for %s, want it disabled", testCase.disabled, safari)
			}
		})
	}
}
