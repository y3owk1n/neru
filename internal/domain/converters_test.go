package domain_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/domain"
)

func TestModeString(t *testing.T) {
	tests := []struct {
		name string
		mode app.Mode
		want string
	}{
		{
			name: "idle mode",
			mode: app.ModeIdle,
			want: "idle",
		},
		{
			name: "hints mode",
			mode: app.ModeHints,
			want: "hints",
		},
		{
			name: "grid mode",
			mode: app.ModeGrid,
			want: "grid",
		},
		{
			name: "scroll mode",
			mode: app.ModeScroll,
			want: domain.ModeNameScroll,
		},
		// The names below are IPC action words and the status `mode` value,
		// so they are spelled out rather than read from the constants.
		{
			name: "recursive grid mode",
			mode: domain.ModeRecursiveGrid,
			want: "recursive_grid",
		},
		{
			name: "bisect mode",
			mode: domain.ModeBisect,
			want: "bisect",
		},
		{
			name: "monitor select mode",
			mode: domain.ModeMonitorSelect,
			want: "monitor_select",
		},
		{
			name: "vim mode",
			mode: domain.ModeVim,
			want: "vim",
		},
		{
			name: "declared mode",
			mode: domain.ModeCustom,
			want: "mode",
		},
		{
			name: "unknown mode",
			mode: app.Mode(999),
			want: domain.UnknownMode,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := domain.ModeString(testCase.mode)
			if got != testCase.want {
				t.Errorf("ModeString(%v) = %q, want %q", testCase.mode, got, testCase.want)
			}
		})
	}
}
