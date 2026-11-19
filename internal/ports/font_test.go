package ports_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/ports"
)

func TestResolveFont_DefaultNoopReturnsInput(t *testing.T) {
	ports.SetFontResolver(nil)
	t.Cleanup(func() { ports.SetFontResolver(nil) })

	if got := ports.ResolveFont("Anything"); got != "Anything" {
		t.Fatalf("expected input to be returned unchanged, got %q", got)
	}

	if got := ports.ResolveFont(""); got != "" {
		t.Fatalf("expected empty input to be returned unchanged, got %q", got)
	}
}
