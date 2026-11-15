package hookref_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/event"
	"github.com/y3owk1n/neru/internal/hookref"
)

// page is shaped like the hooks section: prose around two regions to fill.
const page = "# Hooks\n\nBefore.\n\n" +
	hookref.HooksBeginMarker + "\n\nstale\n\n" + hookref.HooksEndMarker +
	"\n\nBetween.\n\n" +
	hookref.VariablesBeginMarker + "\n\nstale\n\n" + hookref.VariablesEndMarker +
	"\n\nAfter.\n"

func TestRewrite_FillsBothRegionsAndKeepsTheProse(t *testing.T) {
	t.Parallel()

	updated, err := hookref.Rewrite(page)
	if err != nil {
		t.Fatalf("Rewrite() error = %v", err)
	}

	if strings.Contains(updated, "stale") {
		t.Error("the previous contents of a region survived")
	}

	for _, prose := range []string{"Before.", "Between.", "After."} {
		if !strings.Contains(updated, prose) {
			t.Errorf("Rewrite() dropped the prose %q", prose)
		}
	}

	hooks, err := hookref.HooksRegion(updated)
	if err != nil {
		t.Fatalf("HooksRegion() error = %v", err)
	}

	for _, spec := range event.Catalog() {
		if !strings.Contains(hooks, "`"+spec.Hook()+"`") {
			t.Errorf("the hooks table has no row for %s", spec.Hook())
		}
	}

	variables, err := hookref.VariablesRegion(updated)
	if err != nil {
		t.Fatalf("VariablesRegion() error = %v", err)
	}

	if !strings.Contains(variables, "| `NERU_EVENT` | every hook |") {
		t.Errorf("NERU_EVENT is not listed for every hook:\n%s", variables)
	}
}

func TestRewrite_RefusesAPageMissingARegion(t *testing.T) {
	t.Parallel()

	onlyHooks := hookref.HooksBeginMarker + "\n\n" + hookref.HooksEndMarker + "\n"

	_, err := hookref.Rewrite(onlyHooks)
	if err == nil {
		t.Error("Rewrite() accepted a page with no variables region")
	}
}
