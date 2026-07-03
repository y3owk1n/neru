//go:build windows && (amd64 || arm64)

package windows //nolint:testpackage // exercises unexported Win32 argument packing helper

import (
	"image"
	"testing"
)

func TestPackMonitorPoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		point image.Point
		want  uintptr
	}{
		{
			name:  "primary monitor coordinate",
			point: image.Pt(1280, 720),
			want:  uintptr(0x000002D000000500),
		},
		{
			name:  "left of primary monitor coordinate",
			point: image.Pt(-1080, 261),
			want:  uintptr(0x00000105FFFFFBC8),
		},
		{
			name:  "right monitor coordinate",
			point: image.Pt(2846, 261),
			want:  uintptr(0x0000010500000B1E),
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := packMonitorPoint(testCase.point); got != testCase.want {
				t.Fatalf("packMonitorPoint(%v) = %#x, want %#x", testCase.point, got, testCase.want)
			}
		})
	}
}
