package loader

import (
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// The words the fixtures below are built around, named once so two fixtures
// cannot disagree about how one of them is spelled.
const (
	smoothScrollEnabled = "smooth_scroll.enabled"
	hideCursorStep      = "action hide_cursor"
	leftClickStep       = "action left_click"
	visionStrategy      = "vision"
	hideCursorAction    = "hide_cursor"
)

// TestWarnInertWords_SaysItOnce reads the sentence a Linux user is shown, from
// whichever machine the test runs on. The wording is the whole of what the
// load-time half of ADR 0013 delivers, so it is asserted rather than left to a
// platform the maintainer does not develop on.
func TestWarnInertWords_SaysItOnce(t *testing.T) {
	t.Parallel()

	warnings := &config.Warnings{}

	written := config.Written{
		Options: map[string]string{smoothScrollEnabled: "true"},
		Steps:   []string{hideCursorStep},
	}

	inert := warnInertWords(warnings, written, parity.Linux)

	if got := inert.Names(); !slices.Equal(got, []string{smoothScrollEnabled, hideCursorAction}) {
		t.Fatalf("warnInertWords found %v, want [smooth_scroll.enabled hide_cursor]", got)
	}

	messages := warnings.Messages()
	if len(messages) != 1 {
		t.Fatalf("warnInertWords recorded %d warnings, want exactly 1: %q", len(messages), messages)
	}

	for _, want := range []string{
		"2 settings",
		"do nothing on linux",
		smoothScrollEnabled,
		hideCursorAction,
		"neru doctor",
	} {
		if !strings.Contains(messages[0], want) {
			t.Errorf("the warning %q does not mention %q", messages[0], want)
		}
	}
}

// TestWarnInertWords_SaysNothingWhenNothingIsInert keeps the warning off the
// path it does not belong on: a configuration whose every word works here has
// nothing to be told.
func TestWarnInertWords_SaysNothingWhenNothingIsInert(t *testing.T) {
	t.Parallel()

	warnings := &config.Warnings{}

	written := config.Written{Options: map[string]string{"grid.enabled": "true"}}

	if inert := warnInertWords(warnings, written, parity.Linux); len(inert) > 0 {
		t.Errorf("warnInertWords found %v for a configuration that works", inert.Names())
	}

	if messages := warnings.Messages(); len(messages) > 0 {
		t.Errorf("warnInertWords recorded %q for a configuration that works", messages)
	}
}

// TestListInert_StopsListingAndCounts pins the bound. A user with forty inert
// options learns more from a sentence and a command to run than from forty
// paths in a warning.
func TestListInert_StopsListingAndCounts(t *testing.T) {
	t.Parallel()

	many := make(parity.Declaration, 0, listedInertWords+3)

	for index := range listedInertWords + 3 {
		many = append(many, parity.Word{
			Kind: parity.KindOption,
			Name: strings.Repeat("a", index+1),
		})
	}

	listed := listInert(many)
	if !strings.HasSuffix(listed, "and 3 more") {
		t.Errorf("listInert(...) = %q, want it to stop at %d and count the rest",
			listed, listedInertWords)
	}

	if got := strings.Count(listed, ", ") + 1; got != listedInertWords {
		t.Errorf("listInert(...) named %d entries before counting, want %d",
			got, listedInertWords)
	}
}

// TestWrittenConfiguration_ReadsPathsAndSteps pins what the reading takes from
// a decoded file: every key path with the value at it, and every string in the
// tree as a step to read for a vocabulary.
func TestWrittenConfiguration_ReadsPathsAndSteps(t *testing.T) {
	t.Parallel()

	raw := map[string]any{
		"smooth_scroll": map[string]any{"enabled": true, "steps": int64(12)},
		"hints":         map[string]any{"strategy": visionStrategy},
		"hotkeys":       map[string]any{"Ctrl+Shift+H": hideCursorStep},
		"macros":        map[string]any{"twice": []any{leftClickStep, leftClickStep}},
	}

	written := writtenConfiguration(raw, "")

	for path, want := range map[string]string{
		smoothScrollEnabled:   "",
		"smooth_scroll.steps": "",
		"hints.strategy":      visionStrategy,
	} {
		got, wrote := written.Options[path]
		if !wrote {
			t.Errorf("written.Options has no %q", path)

			continue
		}

		if got != want {
			t.Errorf("written.Options[%q] = %q, want %q", path, got, want)
		}
	}

	for _, want := range []string{hideCursorStep, leftClickStep, visionStrategy} {
		if !slices.Contains(written.Steps, want) {
			t.Errorf("written.Steps = %q, want it to contain %q", written.Steps, want)
		}
	}
}
