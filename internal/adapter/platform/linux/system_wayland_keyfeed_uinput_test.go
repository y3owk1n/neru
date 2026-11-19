//go:build linux

package linux

import (
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

func TestUinputModifierCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		modifiers []string
		want      []int
		wantErr   bool
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
		{name: "unknown modifier is not supported", modifiers: []string{"hyper"}, wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := uinputModifierCodes(testCase.modifiers)
			if testCase.wantErr {
				if !derrors.IsCode(err, derrors.CodeNotSupported) {
					t.Fatalf("uinputModifierCodes(%v) error = %v, want CodeNotSupported",
						testCase.modifiers, err)
				}

				return
			}

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
