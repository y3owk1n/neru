//go:build linux && !cgo

package linux

import "testing"

func TestNewFontResolver_ResolvesGenericAliasesToTheLinuxBaseline(t *testing.T) {
	r := NewFontResolver()

	for _, input := range []string{"", "sans"} {
		if got := r.Resolve(input); got != defaultLinuxSans {
			t.Fatalf("Resolve(%q) = %q, want %q", input, got, defaultLinuxSans)
		}
	}
}
