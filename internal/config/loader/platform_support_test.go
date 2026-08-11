package loader_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// inertConfigTOML writes every shape the platform-support reading has to see:
// a whole block that only one platform reads, one value of an option every
// platform recognizes, and a bound action that only one platform performs.
//
// What is inert about it depends on where the test runs, which is the point —
// on macOS none of it is, and the assertions below are written against the
// declaration rather than against a list of paths, so they mean the same thing
// on all three.
const inertConfigTOML = `
[smooth_scroll]
enabled = true
steps = 12

[hints]
strategy = "vision"

[hotkeys]
"Ctrl+Shift+H" = "action hide_cursor"
`

// TestLoadWithValidation_ReportsWhatDoesNothingHere pins the load-time half of
// ADR 0013: a configuration writing words this platform ignores loads, runs,
// and says so once.
func TestLoadWithValidation_ReportsWhatDoesNothingHere(t *testing.T) {
	result, logs := loadWithObservedLogger(t, inertConfigTOML, "")

	if result.ValidationError != nil {
		t.Fatalf("an inert word was refused: %v", result.ValidationError)
	}

	platform, known := parity.Current()
	if !known {
		if len(result.Inert) > 0 {
			t.Errorf("result.Inert = %v on a platform with no column at all", result.Inert.Names())
		}

		return
	}

	want := config.InertWords(config.Written{
		Options: map[string]string{
			"smooth_scroll.enabled": "true",
			"smooth_scroll.steps":   "",
			"hints.strategy":        "vision",
		},
		Steps: []string{"action hide_cursor"},
	}, platform)

	if !slices.Equal(result.Inert.Names(), want.Names()) {
		t.Errorf(
			"the load reported %v as inert on %s, want %v; the load and the "+
				"declaration have to answer this the same way",
			result.Inert.Names(), platform, want.Names(),
		)
	}

	// The settings the user wrote survive: a warning reports, it does not undo.
	if !result.Config.SmoothScroll.Enabled {
		t.Error("smooth_scroll.enabled was cleared, want it left as written")
	}

	if len(want) == 0 {
		if logged := countLogged(logs, "do nothing on "); logged != 0 {
			t.Errorf("nothing is inert on %s, but the warning was logged %d times",
				platform, logged)
		}

		return
	}

	// Once, however many words it names: the finding is one thing to learn.
	logged := countLogged(logs, "in this configuration")
	if logged != 1 {
		t.Errorf("the platform-support warning was logged %d times, want exactly 1", logged)
	}

	if !slices.ContainsFunc(result.Warnings, func(warning string) bool {
		return strings.Contains(warning, string(platform))
	}) {
		t.Errorf(
			"result.Warnings = %q, want one naming %s so `neru config validate` prints it",
			result.Warnings, platform,
		)
	}
}

// TestLoadWithValidation_ReportsNothingForAConfigurationNobodyWrote is the
// other half of the same rule. Every platform ships defaults that include
// options it does not read; warning about those would fire on every daemon that
// has never seen a config file, about lines nobody typed.
func TestLoadWithValidation_ReportsNothingForAConfigurationNobodyWrote(t *testing.T) {
	result, _ := loadWithObservedLogger(t, "[grid]\nenabled = true\n", "")

	if len(result.Inert) > 0 {
		t.Errorf(
			"a configuration writing only grid.enabled reported %v as inert; "+
				"only what somebody wrote is judged",
			result.Inert.Names(),
		)
	}
}

// TestLoadWithValidation_ReadsTheOverrideFileForInertWords pins that `neru
// config set` is read too. The override file is the one layer a user writes
// without opening their config, and a word set there is as written as any
// other.
func TestLoadWithValidation_ReadsTheOverrideFileForInertWords(t *testing.T) {
	result, _ := loadWithObservedLogger(t,
		"[grid]\nenabled = true\n",
		"[smooth_scroll]\nenabled = true\n",
	)

	platform, known := parity.Current()
	if !known {
		return
	}

	want := config.InertWords(
		config.Written{Options: map[string]string{"smooth_scroll.enabled": "true"}},
		platform,
	)

	if !slices.Equal(result.Inert.Names(), want.Names()) {
		t.Errorf(
			"an override writing smooth_scroll.enabled reported %v on %s, want %v",
			result.Inert.Names(), platform, want.Names(),
		)
	}
}

// TestLoadWithValidation_DropsInertFindingsWithARefusedFile keeps the finding
// attached to the configuration it was found in. A refused file is replaced by
// the defaults in full, and reporting words from the file that is not running
// would send a user to fix a line that has no effect on anything.
func TestLoadWithValidation_DropsInertFindingsWithARefusedFile(t *testing.T) {
	result, _ := loadWithObservedLogger(t, `
[smooth_scroll]
enabled = true

[hints]
hint_characters = ""
`, "")

	if result.ValidationError == nil {
		t.Fatal("an empty hints.hint_characters was accepted; this test needs a refused file")
	}

	if len(result.Inert) > 0 {
		t.Errorf(
			"a refused file still reported %v as inert; the configuration now "+
				"running is the defaults",
			result.Inert.Names(),
		)
	}
}
