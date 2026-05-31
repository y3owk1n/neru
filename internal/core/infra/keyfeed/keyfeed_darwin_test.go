//go:build darwin

package keyfeed_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/keyfeed"
)

func TestNormalizeKeyForFeed(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "single uppercase letter",
			input:   "A",
			want:    "Shift+A",
			wantErr: false,
		},
		{
			name:    "single lowercase letter",
			input:   "a",
			want:    "a",
			wantErr: false,
		},
		{
			name:    "uppercase with explicit shift",
			input:   "Shift+A",
			want:    "Shift+A",
			wantErr: false,
		},
		{
			name:    "uppercase with cmd modifier - should NOT inject shift",
			input:   "Cmd+A",
			want:    "Cmd+A",
			wantErr: false,
		},
		{
			name:    "single uppercase with leading space",
			input:   " B",
			want:    "Shift+B",
			wantErr: false,
		},
		{
			name:    "single uppercase with trailing space",
			input:   "B ",
			want:    "Shift+B",
			wantErr: false,
		},
		{
			name:    "named key lowercase",
			input:   "enter",
			want:    "Enter",
			wantErr: false,
		},
		{
			name:    "named key uppercase",
			input:   "Escape",
			want:    "Escape",
			wantErr: false,
		},
		{
			name:    "empty string returns error",
			input:   "",
			want:    "",
			wantErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := keyfeed.NormalizeKeyForFeed(testCase.input)

			if testCase.wantErr {
				if err == nil {
					t.Errorf("NormalizeKeyForFeed(%q) expected error, got nil", testCase.input)
				}

				return
			}

			if err != nil {
				t.Errorf("NormalizeKeyForFeed(%q) unexpected error: %v", testCase.input, err)

				return
			}

			if got != testCase.want {
				t.Errorf(
					"NormalizeKeyForFeed(%q) = %q, want %q",
					testCase.input,
					got,
					testCase.want,
				)
			}
		})
	}
}
