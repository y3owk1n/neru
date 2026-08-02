package keyfeed_test

import (
	"context"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/keyfeed"
	"github.com/y3owk1n/neru/internal/derrors"
)

const shiftA = "Shift+A"

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
			want:    shiftA,
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
			input:   shiftA,
			want:    shiftA,
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

// TestAdapter_FeedRejectsEmptyKey pins the shared validation: it runs on every
// platform because normalization is shared code, unlike the injection behind it.
func TestAdapter_FeedRejectsEmptyKey(t *testing.T) {
	adapter := keyfeed.NewAdapter(nil)

	err := adapter.Feed(t.Context(), "   ")
	if err == nil {
		t.Fatal("Feed(\"   \") = nil, want an error")
	}

	if !derrors.IsCode(err, derrors.CodeInvalidInput) {
		t.Errorf("Feed(\"   \") code = %v, want CodeInvalidInput", err)
	}
}

// TestAdapter_FeedHonorsCanceledContext pins that the port contract's context
// is actually observed, so a canceled chain stops feeding keys.
func TestAdapter_FeedHonorsCanceledContext(t *testing.T) {
	adapter := keyfeed.NewAdapter(nil)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := adapter.Feed(ctx, "a")
	if !derrors.IsCode(err, derrors.CodeContextCanceled) {
		t.Errorf("Feed on canceled context = %v, want CodeContextCanceled", err)
	}
}

// TestNormalizeKeyForFeed_ExplicitModifierSuppressesShiftInjection covers the
// case the table cannot: an uppercase letter already carrying a modifier must
// not have Shift added on top of it.
//
// It asserts the property rather than an exact string because the rendered
// modifier name is platform-specific — config.CanonicalHotkeyForPlatform maps
// "Cmd" to "Cmd" on macOS and "Super" elsewhere. Pinning "Cmd+A" here is what
// made this test pass on macOS and fail on Linux.
func TestNormalizeKeyForFeed_ExplicitModifierSuppressesShiftInjection(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"Cmd+A", "Ctrl+A", "Alt+A", "Primary+A"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			got, err := keyfeed.NormalizeKeyForFeed(input)
			if err != nil {
				t.Fatalf("NormalizeKeyForFeed(%q) error = %v, want nil", input, err)
			}

			if strings.Contains(got, "Shift") {
				t.Errorf(
					"NormalizeKeyForFeed(%q) = %q; Shift must not be injected when "+
						"the key already carries a modifier",
					input,
					got,
				)
			}

			if !strings.HasSuffix(got, "+A") {
				t.Errorf("NormalizeKeyForFeed(%q) = %q, want it to still end in +A", input, got)
			}
		})
	}
}
