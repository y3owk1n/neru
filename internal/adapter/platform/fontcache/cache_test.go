package fontcache_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform/fontcache"
)

func TestResolver_ResolvesOncePerWrittenName(t *testing.T) {
	calls := 0
	resolver := fontcache.New(func(family string) string {
		calls++

		return strings.TrimSpace(family)
	})

	for range 3 {
		if got := resolver.Resolve("JetBrains Mono"); got != "JetBrains Mono" {
			t.Fatalf("Resolve() = %q, want %q", got, "JetBrains Mono")
		}
	}

	if calls != 1 {
		t.Fatalf("resolved %d times, want 1", calls)
	}
}

func TestResolver_AnswerNeverDependsOnWhatWasAskedBefore(t *testing.T) {
	// The rule this package exists for: an answer is a function of the name
	// asked for and nothing else. Remembering an answer under a normalised
	// name while storing the written one would hand the second caller the
	// first caller's spelling, which is the order-dependent behavior #1293
	// removed.
	spellings := []string{"Arial", "ARIAL", "arial", "  Arial  "}

	for _, first := range spellings {
		t.Run(first, func(t *testing.T) {
			resolver := fontcache.New(strings.TrimSpace)

			resolver.Resolve(first)

			for _, second := range spellings {
				want := strings.TrimSpace(second)

				if got := resolver.Resolve(second); got != want {
					t.Fatalf(
						"after Resolve(%q), Resolve(%q) = %q, want %q",
						first,
						second,
						got,
						want,
					)
				}
			}
		})
	}
}

func TestResolver_SecondSpellingIsResolvedOnItsOwn(t *testing.T) {
	// The documented price of remembering a name as written: a second spelling
	// of the same family is a second name, resolved rather than answered from
	// the first one's entry.
	calls := 0
	resolver := fontcache.New(func(family string) string {
		calls++

		return strings.TrimSpace(family)
	})

	resolver.Resolve("Arial")
	resolver.Resolve("ARIAL")

	if calls != 2 {
		t.Fatalf("resolved %d times for two spellings, want 2", calls)
	}
}

func TestResolver_ConcurrentResolveIsSafe(t *testing.T) {
	var mu sync.Mutex

	resolver := fontcache.New(func(family string) string {
		mu.Lock()
		defer mu.Unlock()

		return strings.TrimSpace(family)
	})

	var waitGroup sync.WaitGroup

	for _, family := range []string{"Arial", "ARIAL", "Menlo", "  Menlo  "} {
		for range 8 {
			waitGroup.Go(func() {
				want := strings.TrimSpace(family)

				if got := resolver.Resolve(family); got != want {
					t.Errorf("Resolve(%q) = %q, want %q", family, got, want)
				}
			})
		}
	}

	waitGroup.Wait()
}
