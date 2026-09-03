//go:build linux

package linux

import (
	"os"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/ports"
)

// TestScrollCapability_UinputUnwritableNamesTheFix pins what `neru doctor`
// prints when the uinput wheel cannot be opened: a downgrade rather than a
// green row, carrying the open error and the device to make writable.
func TestScrollCapability_UinputUnwritableNamesTheFix(t *testing.T) {
	declared := ports.FeatureCapability{Status: ports.FeatureStatusSupported, Detail: "declared"}

	tests := []struct {
		name       string
		uinputErr  error
		wantStatus ports.FeatureStatus
		wantDetail []string
	}{
		{
			name:       "uinput writable keeps the declared capability",
			uinputErr:  nil,
			wantStatus: ports.FeatureStatusSupported,
			wantDetail: []string{"declared"},
		},
		{
			name:       "uinput unwritable downgrades with the reason and the device",
			uinputErr:  &os.PathError{Op: "open", Path: uinputDevicePath, Err: os.ErrPermission},
			wantStatus: ports.FeatureStatusStub,
			wantDetail: []string{"permission denied", uinputDevicePath, "virtual pointer"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := scrollCapability(declared, testCase.uinputErr)

			if got.Status != testCase.wantStatus {
				t.Fatalf("Status = %q, want %q", got.Status, testCase.wantStatus)
			}

			for _, want := range testCase.wantDetail {
				if !strings.Contains(got.Detail, want) {
					t.Errorf("Detail %q does not mention %q", got.Detail, want)
				}
			}
		})
	}
}
