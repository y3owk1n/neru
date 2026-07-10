//go:build darwin

package axobserver

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/axnotify"
)

// TestToDarwinMaskCoversEveryBit pins the second half of the
// name -> package bit -> native bit chain. TestNotificationBitsCoverVocabulary
// guards that every axnotify name has a package bit; this guards that every
// package bit maps to a non-zero native notification. Add a notification to the
// vocabulary and the registry but forget toDarwinMask, and MaskFromNames would
// produce a bit the darwin backend silently drops, subscribing to nothing with
// no error. This turns that silent gap into a test failure.
func TestToDarwinMaskCoversEveryBit(t *testing.T) {
	for _, name := range axnotify.AllNames() {
		mask, err := MaskFromNames([]string{name})
		if err != nil {
			t.Fatalf("MaskFromNames(%q): %v", name, err)
		}

		if native := toDarwinMask(mask); native == 0 {
			t.Errorf("notification %q (bit %d) maps to the zero native mask; toDarwinMask is missing it",
				name, mask)
		}
	}
}
