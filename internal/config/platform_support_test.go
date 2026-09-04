package config_test

import (
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// The words the fixtures below are built around, named once so two fixtures
// cannot disagree about how one of them is spelled.
const (
	screenShareHide     = "general.hide_overlay_in_screen_share"
	smoothScrollEnabled = "smooth_scroll.enabled"
	hintsStrategy       = "hints.strategy"
	hideCursorStep      = "action hide_cursor"
	hideCursorAction    = "hide_cursor"
	visionStrategy      = "vision"
	visionMinConfidence = "hints.vision.minimum_confidence"
	trueValue           = "true"
)

// TestPlatformSupport_DeclaresTheKnownNarrowColumns pins the members ADR 0013
// named, so a well-meaning cleanup that widened one back to every platform has
// to argue with a test rather than with nobody.
func TestPlatformSupport_DeclaresTheKnownNarrowColumns(t *testing.T) {
	t.Parallel()

	declaration := config.PlatformSupport()

	tests := []struct {
		name  string
		path  string
		value string
		want  parity.Platforms
	}{
		{
			"hiding the overlay from a screen share is a Quartz concept",
			screenShareHide, "",
			parity.Platforms{parity.Darwin},
		},
		{
			"smooth scroll animates on macOS and Linux, not Windows",
			smoothScrollEnabled, "",
			parity.Platforms{parity.Darwin, parity.Linux},
		},
		{
			"rectangle detection has no OCR answer",
			"hints.vision.detect_rectangles", "",
			parity.Platforms{parity.Darwin},
		},
		{
			"Windows.Media.Ocr reports no confidence, so the floor is inert there",
			visionMinConfidence, "",
			parity.Platforms{parity.Darwin, parity.Linux},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			word, found := declaration.Lookup(parity.KindOption, test.path, test.value)
			if !found {
				t.Fatalf("no declaration for option %q with value %q", test.path, test.value)
			}

			if !slices.Equal(word.Platforms, test.want) {
				t.Errorf("%q is declared on %v, want %v", test.path, word.Platforms, test.want)
			}

			if word.Note == "" {
				t.Errorf("%q carries a narrow column with no note saying why", test.path)
			}
		})
	}
}

// TestPlatformSupport_DeclaresTheOptionBehindAValue is the reason the strategy
// option resolves at all on a platform with no vision engine: the option is
// recognized everywhere and only one value of it is not.
func TestPlatformSupport_DeclaresTheOptionBehindAValue(t *testing.T) {
	t.Parallel()

	word, found := config.PlatformSupport().Lookup(parity.KindOption, hintsStrategy, "")
	if !found {
		t.Fatal("hints.strategy itself is not declared")
	}

	if !word.Platforms.Everywhere() {
		t.Errorf("hints.strategy is declared as %v; the option is written everywhere, "+
			"and only its vision value is not", word.Platforms)
	}
}

func TestInertWords_Options(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		written map[string]string
		target  parity.Platform
		want    []string
	}{
		{
			name:    "an option nobody wrote is not reported",
			written: map[string]string{"grid.enabled": trueValue},
			target:  parity.Linux,
			want:    nil,
		},
		{
			name:    "an option written where it is inert is reported",
			written: map[string]string{screenShareHide: trueValue},
			target:  parity.Linux,
			want:    []string{screenShareHide},
		},
		{
			name:    "the same option is not reported where it works",
			written: map[string]string{screenShareHide: trueValue},
			target:  parity.Darwin,
			want:    nil,
		},
		{
			name:    "the vision strategy is silent now that every platform has an engine",
			written: map[string]string{hintsStrategy: visionStrategy},
			target:  parity.Windows,
			want:    nil,
		},
		{
			name:    "a floor the Windows engine cannot honor is reported there",
			written: map[string]string{visionMinConfidence: "0.5"},
			target:  parity.Windows,
			want:    []string{visionMinConfidence},
		},
		{
			name:    "an option inert only on Windows is silent on Linux",
			written: map[string]string{smoothScrollEnabled: "true"},
			target:  parity.Linux,
			want:    nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := config.InertWords(config.Written{Options: test.written}, test.target).Names()
			if !slices.Equal(got, test.want) {
				t.Errorf("InertWords(%v) = %v, want %v", test.written, got, test.want)
			}
		})
	}
}

func TestInertWords_Steps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		steps  []string
		target parity.Platform
		want   []string
	}{
		{
			name:   "an action that works everywhere is not reported",
			steps:  []string{"action left_click"},
			target: parity.Linux,
			want:   nil,
		},
		{
			name:   "a darwin-only action is reported where it does nothing",
			steps:  []string{hideCursorStep},
			target: parity.Linux,
			want:   []string{hideCursorAction},
		},
		{
			name:   "the same action is not reported on darwin",
			steps:  []string{hideCursorStep},
			target: parity.Darwin,
			want:   nil,
		},
		{
			name:   "an action written twice is reported once",
			steps:  []string{hideCursorStep, hideCursorStep},
			target: parity.Linux,
			want:   []string{hideCursorAction},
		},
		{
			name:   "horizontal scroll is silent now that Windows injects both axes",
			steps:  []string{"action scroll_right"},
			target: parity.Windows,
			want:   nil,
		},
		{
			name:   "the vision flags are silent now that every platform has an engine",
			steps:  []string{"hints --strategy=vision --split-word"},
			target: parity.Windows,
			want:   nil,
		},
		{
			name:   "a mode command with no narrow flag is silent",
			steps:  []string{"hints --toggle"},
			target: parity.Windows,
			want:   nil,
		},
		{
			name:   "a step that is not a command at all is silent",
			steps:  []string{"exec notify-send nothing", ""},
			target: parity.Linux,
			want:   nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := config.InertWords(config.Written{Steps: test.steps}, test.target).Names()
			if !slices.Equal(got, test.want) {
				t.Errorf("InertWords(%v) = %v, want %v", test.steps, got, test.want)
			}
		})
	}
}

// TestInertWords_LeavesTheShippedBindingsSilentWhereTheyWork keeps the warning
// about what somebody wrote. The shipped scroll bindings name scroll_left and
// scroll_right, which inject nothing on Windows — a Known Gaps entry rather than
// a line a user can go and fix — which is why the reading is of the user's files
// and not of the merged configuration, and why Windows is not asserted here.
func TestInertWords_LeavesTheShippedBindingsSilentWhereTheyWork(t *testing.T) {
	t.Parallel()

	defaults := config.DefaultConfig()

	var steps []string

	for _, actions := range defaults.Hotkeys.Bindings {
		steps = append(steps, actions...)
	}

	for _, target := range []parity.Platform{parity.Darwin, parity.Linux} {
		found := config.InertWords(config.Written{Steps: steps}, target).Names()
		if len(found) > 0 {
			t.Errorf(
				"the default global bindings write %v, which does nothing on %s; "+
					"either the default is wrong or the word's column is",
				found, target,
			)
		}
	}
}
