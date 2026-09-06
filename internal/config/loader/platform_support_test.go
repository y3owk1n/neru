package loader_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
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

// passthroughConfigTOML turns modifier passthrough on and writes both of the
// options that mean nothing without it, which is the whole set the X11 backend
// cannot honor.
const passthroughConfigTOML = `
[general]
passthrough_unbounded_keys = true
should_exit_after_passthrough = true
passthrough_unbounded_keys_blacklist = ["Ctrl+C"]
`

// passthroughOptions are the words the two tests below expect, in declaration
// order.
var passthroughOptions = []string{
	"general.passthrough_unbounded_keys",
	"general.passthrough_unbounded_keys_blacklist",
	"general.should_exit_after_passthrough",
}

// onX11 stands in for the backend detection a composition root does: it wires
// the X11 limit onto the loader the way main.go does when the display server
// it detected is X11. Faked rather than detected so the sentence an X11 user is
// shown can be read from any machine (#1613).
func onX11(svc *loader.Service) *loader.Service {
	return svc.WithBackendInert(config.X11InertWords)
}

// TestLoadWithValidation_WarnsOnceWhenTheX11BackendCannotPassThrough is the
// load-time half of #1613: passthrough on the X11 backend is a word that means
// nothing, and the file says so once, naming the options, the backend and why.
func TestLoadWithValidation_WarnsOnceWhenTheX11BackendCannotPassThrough(t *testing.T) {
	result, logs := loadWithObservedLogger(t, passthroughConfigTOML, "", onX11)

	if result.ValidationError != nil {
		t.Fatalf("passthrough on X11 was refused: %v", result.ValidationError)
	}

	for _, want := range passthroughOptions {
		if !slices.Contains(result.Inert.Names(), want) {
			t.Errorf("result.Inert = %v, want it to carry %s for `neru doctor`",
				result.Inert.Names(), want)
		}
	}

	var about []string

	for _, warning := range result.Warnings {
		if strings.Contains(warning, "passthrough_unbounded_keys") {
			about = append(about, warning)
		}
	}

	if len(about) != 1 {
		t.Fatalf("got %d warnings about passthrough, want exactly 1: %q", len(about), about)
	}

	for _, want := range append(
		[]string{"3 settings", "X11", "XGrabKeyboard", "the daemon runs"},
		passthroughOptions...,
	) {
		if !strings.Contains(about[0], want) {
			t.Errorf("the warning %q does not mention %q", about[0], want)
		}
	}

	if logged := countLogged(logs, "XGrabKeyboard"); logged != 1 {
		t.Errorf("the X11 passthrough warning was logged %d times, want exactly 1", logged)
	}

	// A warning reports, it does not undo: the option stays as written for the
	// day the same file is loaded under Wayland.
	if !result.Config.General.PassthroughUnboundedKeys {
		t.Error("passthrough_unbounded_keys was cleared, want it left as written")
	}
}

// TestLoadWithValidation_SaysNothingAboutPassthroughOffX11 keeps the warning
// off every backend that honors the option. No root wires the X11 limit under
// Wayland, so the loader has nothing to say about passthrough there.
func TestLoadWithValidation_SaysNothingAboutPassthroughOffX11(t *testing.T) {
	result, logs := loadWithObservedLogger(t, passthroughConfigTOML, "")

	for _, name := range passthroughOptions {
		if slices.Contains(result.Inert.Names(), name) {
			t.Errorf("result.Inert = %v names %s off X11, where the option works",
				result.Inert.Names(), name)
		}
	}

	for _, warning := range result.Warnings {
		if strings.Contains(warning, "passthrough_unbounded_keys") {
			t.Errorf("result.Warnings carries %q off X11, want nothing about passthrough", warning)
		}
	}

	if logged := countLogged(logs, "XGrabKeyboard"); logged != 0 {
		t.Errorf("the X11 passthrough warning was logged %d times off X11, want 0", logged)
	}
}
