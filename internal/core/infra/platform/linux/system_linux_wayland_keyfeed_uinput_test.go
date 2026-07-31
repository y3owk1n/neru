//go:build linux

//nolint:testpackage // Exercises the unexported uinput modifier-mapping helper directly.
package linux

import (
	"testing"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

func TestUinputModifierCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		modifiers []string
		want      []int
	}{
		{name: "none", modifiers: nil, want: []int{}},
		{name: "ctrl", modifiers: []string{modNameCtrl}, want: []int{keycodeLeftCtrl}},
		{
			name:      "ctrl+shift preserves order",
			modifiers: []string{modNameCtrl, modNameShift},
			want:      []int{keycodeLeftCtrl, keycodeLeftShift},
		},
		{
			name:      "all four",
			modifiers: []string{modNameShift, modNameCtrl, modNameAlt, modNameCmd},
			want:      []int{keycodeLeftShift, keycodeLeftCtrl, keycodeLeftAlt, keycodeLeftMeta},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := uinputModifierCodes(testCase.modifiers)
			if err != nil {
				t.Fatalf("uinputModifierCodes(%v) returned error: %v", testCase.modifiers, err)
			}

			if len(got) != len(testCase.want) {
				t.Fatalf("length = %d, want %d (%v)", len(got), len(testCase.want), got)
			}

			for i := range got {
				if got[i] != testCase.want[i] {
					t.Fatalf("codes = %v, want %v", got, testCase.want)
				}
			}
		})
	}
}

func TestUinputModifierCodesUnknown(t *testing.T) {
	t.Parallel()

	_, err := uinputModifierCodes([]string{"hyper"})
	if err == nil {
		t.Fatal("expected error for unknown modifier, got nil")
	}

	if !derrors.IsCode(err, derrors.CodeNotSupported) {
		t.Fatalf("error code = %v, want CodeNotSupported", err)
	}
}
