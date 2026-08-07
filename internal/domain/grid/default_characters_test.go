package grid_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/grid"
)

// TestDefaultCharacters_OmitsO pins the one thing the constant itself cannot
// say: `o` is left out on purpose, because it is hard to tell from `0` at label
// size. Sharing the constant makes the config default and the fallback the same
// set; this is what keeps that set from quietly growing an `o` back.
func TestDefaultCharacters_OmitsO(t *testing.T) {
	if strings.ContainsAny(grid.DefaultCharacters, "oO") {
		t.Errorf("DefaultCharacters = %q, want no o", grid.DefaultCharacters)
	}
}
