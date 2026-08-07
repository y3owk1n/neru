//go:build linux && !cgo

package linux

import "testing"

func TestPassthroughResolver_CachesByFamily(t *testing.T) {
	r := passthroughResolver{}

	// Repeated calls with the same input should not mutate behaviour.
	for range 3 {
		if got := r.Resolve("sans", true); got != defaultLinuxSans {
			t.Fatalf("expected generic alias to resolve to %q, got %q", defaultLinuxSans, got)
		}
	}
}

func TestPassthroughResolver_EmptyDefaultsToSans(t *testing.T) {
	r := passthroughResolver{}

	if got := r.Resolve("", false); got != defaultLinuxSans {
		t.Fatalf("expected empty input to resolve to %q, got %q", defaultLinuxSans, got)
	}
}
