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

	tests := []struct {
		name  string
		label string
	}{
		{name: "hint label", label: "AB"},
		{name: "single key", label: "r"},
		{name: "grid coordinate", label: "AED"},
	}

	seen := map[unsafe.Pointer]string{}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			first := cache.Get(testCase.label)
			if first == nil {
				t.Fatalf("Get(%q) = nil, want a C string", testCase.label)
			}

			if again := cache.Get(testCase.label); again != first {
				t.Errorf("second Get(%q) = %p, want the cached %p", testCase.label, again, first)
			}

			if owner, taken := seen[first]; taken {
				t.Errorf("Get(%q) returned the pointer cached for %q", testCase.label, owner)
			}

			seen[first] = testCase.label

			assertCString(t, first, testCase.label)
		})
	}

	cache.Free()

	assertCString(t, cache.Get("AB"), "AB")
}

// assertCString checks that ptr holds want followed by a NUL byte.
func assertCString(t *testing.T, ptr unsafe.Pointer, want string) {
	t.Helper()

	if got := unsafe.String((*byte)(ptr), len(want)); got != want {
		t.Errorf("C string reads %q, want %q", got, want)
	}

	if terminator := *(*byte)(unsafe.Add(ptr, len(want))); terminator != 0 {
		t.Errorf("C string for %q is not NUL-terminated, byte after it is %d", want, terminator)
	}
}
