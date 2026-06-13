package ports_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// fakeResolver is a deterministic ports.FontResolver used to exercise the
// global accessor and the noop default behavior in isolation.
type fakeResolver struct {
	calls int
	last  string
}

func (f *fakeResolver) Resolve(family string, _ bool) string {
	f.calls++
	f.last = family

	return "RESOLVED:" + family
}

func TestResolveFont_DefaultNoopReturnsInput(t *testing.T) {
	ports.SetFontResolver(nil)
	t.Cleanup(func() { ports.SetFontResolver(nil) })

	if got := ports.ResolveFont("Anything", true); got != "Anything" {
		t.Fatalf("expected input to be returned unchanged, got %q", got)
	}

	if got := ports.ResolveFont("", false); got != "" {
		t.Fatalf("expected empty input to be returned unchanged, got %q", got)
	}
}

func TestResolveFont_DispatchesToInstalledResolver(t *testing.T) {
	fake := &fakeResolver{}

	ports.SetFontResolver(fake)
	t.Cleanup(func() { ports.SetFontResolver(nil) })

	if got := ports.ResolveFont("JetBrains Mono", true); got != "RESOLVED:JetBrains Mono" {
		t.Fatalf("expected resolver to be called, got %q", got)
	}

	if fake.calls != 1 {
		t.Fatalf("expected resolver to be called exactly once, got %d", fake.calls)
	}

	if fake.last != "JetBrains Mono" {
		t.Fatalf("expected resolver to receive input family, got %q", fake.last)
	}
}

func TestResolveFont_NilResetRestoresNoop(t *testing.T) {
	ports.SetFontResolver(&fakeResolver{})
	ports.SetFontResolver(nil)
	t.Cleanup(func() { ports.SetFontResolver(nil) })

	if got := ports.ResolveFont("X", false); got != "X" {
		t.Fatalf("expected noop default after reset, got %q", got)
	}
}
