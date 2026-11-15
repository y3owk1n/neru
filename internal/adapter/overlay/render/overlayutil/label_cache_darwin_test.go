//go:build darwin

package overlayutil_test

import (
	"testing"
	"unsafe"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/overlayutil"
)

// TestLabelCache_Get_ReturnsOneCStringPerLabelUntilFree pins what the overlays
// rely on. The bridge keeps label pointers between draws, so asking for a label
// again must return the pointer it already holds, and that pointer must hold
// the label as a NUL-terminated C string.
func TestLabelCache_Get_ReturnsOneCStringPerLabelUntilFree(t *testing.T) {
	var cache overlayutil.LabelCache
	t.Cleanup(cache.Free)

	first := cache.Get("AB")
	if first == nil {
		t.Fatal(`Get("AB") = nil, want a C string`)
	}

	if again := cache.Get("AB"); again != first {
		t.Errorf(`second Get("AB") = %p, want the cached %p`, again, first)
	}

	if other := cache.Get("CD"); other == first {
		t.Errorf(`Get("CD") returned the pointer cached for "AB"`)
	}

	if got := unsafe.String((*byte)(first), 2); got != "AB" {
		t.Errorf("cached C string reads %q, want %q", got, "AB")
	}

	if terminator := *(*byte)(unsafe.Add(first, 2)); terminator != 0 {
		t.Errorf("cached C string is not NUL-terminated, byte after it is %d", terminator)
	}

	cache.Free()

	if got := unsafe.String((*byte)(cache.Get("AB")), 2); got != "AB" {
		t.Errorf("Get after Free reads %q, want a fresh %q", got, "AB")
	}
}
