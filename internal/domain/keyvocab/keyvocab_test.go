package keyvocab_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

const (
	wantCmd    = "cmd"
	wantKeyUpJ = "__keyup_j"
)

func TestNormalizeKey_CanonicalSpellings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "return", in: "return", want: keyReturn},
		{name: "enter aliases to Return", in: nameEnter, want: keyReturn},
		{name: "enter with modifiers", in: "ctrl+" + nameEnter, want: "ctrl+" + keyReturn},
		{name: "esc aliases to Escape", in: nameEsc, want: keyEscape},
		{name: "backspace maps to Delete", in: nameBackspace, want: keyDelete},
		{name: "single rune lowercased", in: "A", want: "a"},
		{name: "modifiers pass through", in: "cmd+shift+K", want: "cmd+shift+k"},
		{name: "named keys keep case", in: keyLeft, want: keyLeft},
		{name: "empty", in: "", want: ""},
		{name: "whitespace only", in: "   ", want: ""},
		// Every named key gets its display spelling, not just the handful the
		// normalizer used to special-case.
		{name: namePageDown, in: namePageDown, want: keyPageDown},
		{name: "home", in: "HOME", want: "Home"},
		{name: "function key", in: "f13", want: "F13"},
		{name: nameInsert, in: nameInsert, want: keyInsert},
		{name: "unknown multi-rune passes through", in: "Fn", want: "Fn"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := keyvocab.NormalizeKey(testCase.in); got != testCase.want {
				t.Errorf("NormalizeKey(%q) = %q, want %q", testCase.in, got, testCase.want)
			}
		})
	}
}

func TestCanonicalModifier_Aliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: wantCmd, want: wantCmd},
		{in: "command", want: wantCmd},
		{in: "super", want: wantCmd},
		{in: "meta", want: wantCmd},
		{in: "win", want: wantCmd},
		{in: "Shift", want: "shift"},
		{in: "option", want: "alt"},
		{in: "control", want: "ctrl"},
		{in: "hyper", want: ""},
		{in: "", want: ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.in, func(t *testing.T) {
			t.Parallel()

			if got := keyvocab.CanonicalModifier(testCase.in); got != testCase.want {
				t.Errorf("CanonicalModifier(%q) = %q, want %q", testCase.in, got, testCase.want)
			}
		})
	}
}

func TestKeyUpEvent_UsesNormalizedBaseKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain key", in: "j", want: wantKeyUpJ},
		{name: "strips modifiers", in: "cmd+shift+J", want: wantKeyUpJ},
		{name: "named key normalized", in: nameEnter, want: keyvocab.KeyUpPrefix + keyReturn},
		{name: "empty", in: "", want: ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := keyvocab.KeyUpEvent(testCase.in); got != testCase.want {
				t.Errorf("KeyUpEvent(%q) = %q, want %q", testCase.in, got, testCase.want)
			}
		})
	}
}

func TestModifierToggleEvent_RoundTripsThroughParse(t *testing.T) {
	t.Parallel()

	for _, modifier := range []string{wantCmd, "shift", "alt", "ctrl"} {
		for _, isDown := range []bool{true, false} {
			event := keyvocab.ModifierToggleEvent(modifier, isDown)

			gotModifier, gotDown, ok := keyvocab.ParseModifierToggle(event)
			if !ok {
				t.Fatalf("ParseModifierToggle(%q) not ok", event)
			}

			if gotModifier != modifier || gotDown != isDown {
				t.Errorf(
					"round trip of (%s, %t) = (%s, %t)",
					modifier,
					isDown,
					gotModifier,
					gotDown,
				)
			}
		}
	}
}

func TestModifierToggleEvent_AliasAndInvalid(t *testing.T) {
	t.Parallel()

	if got := keyvocab.ModifierToggleEvent("option", true); got != "__modifier_alt_down" {
		t.Errorf("ModifierToggleEvent(option, down) = %q, want __modifier_alt_down", got)
	}

	if got := keyvocab.ModifierToggleEvent("hyper", true); got != "" {
		t.Errorf("ModifierToggleEvent(hyper, down) = %q, want empty", got)
	}
}

func TestParseModifierToggle_Invalid(t *testing.T) {
	t.Parallel()

	for _, event := range []string{
		"",
		"j",
		"__keyup_j",
		"__modifier_",
		"__modifier_hyper_down",
		"__modifier_cmd_sideways",
		"__modifier_cmd",
	} {
		if _, _, ok := keyvocab.ParseModifierToggle(event); ok {
			t.Errorf("ParseModifierToggle(%q) ok = true, want false", event)
		}
	}
}
