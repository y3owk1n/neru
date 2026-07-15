//go:build darwin

package axobserver

import "testing"

// TestToDarwinMaskCoversEveryBit guards that every defined notification bit maps
// to a non-zero native notification. Add a bit to the vocabulary but forget
// toDarwinMask, and the darwin backend would silently drop it, subscribing to
// nothing with no error. This turns that silent gap into a test failure.
func TestToDarwinMaskCoversEveryBit(t *testing.T) {
	for bit := Mask(1); bit != 0 && bit <= highestBit; bit <<= 1 {
		if native := toDarwinMask(bit); native == 0 {
			t.Errorf("bit %#x maps to the zero native mask; toDarwinMask is missing it", bit)
		}
	}
}
