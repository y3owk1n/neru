package action_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// namedPredicate pairs a predicate with the single action name it must accept.
type namedPredicate struct {
	name      action.Name
	predicate func(string) bool
}

func namedPredicates() []namedPredicate {
	return []namedPredicate{
		{action.NameReset, action.IsResetAction},
		{action.NameBackspace, action.IsBackspaceAction},
		{action.NameWaitForModeExit, action.IsWaitForModeExitAction},
		{action.NameSaveCursorPos, action.IsSaveCursorPosAction},
		{action.NameRestoreCursorPos, action.IsRestoreCursorPosAction},
		{action.NameHideCursor, action.IsHideCursorAction},
		{action.NameShowCursor, action.IsShowCursorAction},
		{action.NameMoveMonitor, action.IsMoveMonitorAction},
		{action.NameCycleHint, action.IsCycleHintAction},
		{action.NameSearchHints, action.IsSearchHintsAction},
		{action.NameFeed, action.IsFeedAction},
	}
}

// TestActionPredicates_MatchExactlyTheirOwnName runs every predicate against
// every known action name plus a few near-misses, and requires each to accept
// its own name and reject all others.
//
// The Is<X>Action helpers are the routing table for CLI and IPC action
// dispatch: each one claims a single action name, and the caller runs whichever
// branch says yes. They are one-liners, which is exactly why they were
// untested — and exactly why a slip in one is invisible. An inverted comparison
// makes a predicate answer "yes" for every action except its own, silently
// routing every unrelated action into that branch.
//
// Testing them as a cross-product (each predicate against every name) pins both
// halves of the contract: it matches its own name, and it matches nothing else.
func TestActionPredicates_MatchExactlyTheirOwnName(t *testing.T) {
	predicates := namedPredicates()

	// The candidate set is every known name, plus the names the predicates
	// claim (in case one is not in knownNames), plus values that should never
	// match anything.
	candidates := make(map[action.Name]bool)

	for _, name := range action.KnownNames() {
		candidates[name] = true
	}

	for _, entry := range predicates {
		candidates[entry.name] = true
	}

	for _, extra := range []action.Name{"", "not_an_action", "RESET", "reset ", " reset"} {
		candidates[extra] = true
	}

	for _, entry := range predicates {
		t.Run(string(entry.name), func(t *testing.T) {
			if !entry.predicate(string(entry.name)) {
				t.Errorf("predicate for %q rejected its own name", entry.name)
			}

			for candidate := range candidates {
				if candidate == entry.name {
					continue
				}

				if entry.predicate(string(candidate)) {
					t.Errorf(
						"predicate for %q also matched %q; it must accept only its own name",
						entry.name, candidate,
					)
				}
			}
		})
	}
}

// TestActionPredicates_AreMutuallyExclusive states the property the dispatch
// code relies on directly: for any given action name at most one predicate
// fires, so the dispatcher cannot take two branches for one action.
func TestActionPredicates_AreMutuallyExclusive(t *testing.T) {
	predicates := namedPredicates()

	for _, name := range action.KnownNames() {
		var matched []action.Name

		for _, entry := range predicates {
			if entry.predicate(string(name)) {
				matched = append(matched, entry.name)
			}
		}

		if len(matched) > 1 {
			t.Errorf("action %q matched %d predicates (%v), want at most 1",
				name, len(matched), matched)
		}
	}
}

// TestIsScrollSubAction_CoversEveryScrollNameAndNothingElse pins the boundary
// between scroll sub-actions (CLI/IPC only) and the mode-compatible action set.
// The two are dispatched down different paths, so a name landing on the wrong
// side of this predicate is silently unroutable.
func TestIsScrollSubAction_CoversEveryScrollNameAndNothingElse(t *testing.T) {
	for _, name := range action.KnownNames() {
		// Every mode-compatible known name is by definition not a scroll
		// sub-action; the doc comment on KnownNames draws exactly that line.
		if action.IsScrollSubAction(string(name)) {
			t.Errorf("IsScrollSubAction(%q) = true, but %q is a mode-compatible known name",
				name, name)
		}
	}

	scrollNames := []string{
		"scroll_up", "scroll_down", "scroll_left", "scroll_right",
	}

	for _, name := range scrollNames {
		if !action.IsScrollSubAction(name) {
			t.Errorf("IsScrollSubAction(%q) = false, want true", name)
		}

		// A scroll sub-action must still be a known name overall, or the CLI
		// would reject it before dispatch ever sees it.
		if !action.IsKnownName(action.Name(name)) {
			t.Errorf("IsKnownName(%q) = false, but it is a dispatchable scroll sub-action", name)
		}
	}

	for _, name := range []string{"", "scroll", "scroll_diagonal", "left_click"} {
		if action.IsScrollSubAction(name) {
			t.Errorf("IsScrollSubAction(%q) = true, want false", name)
		}
	}
}

// TestModeActionNamesString_ListsExactlyTheMouseButtonActions checks the help
// text stays derived from the same predicate the CLI validates against. If the
// two drift, `neru mode --help` advertises actions that are then rejected, or
// omits ones that work.
func TestModeActionNamesString_ListsExactlyTheMouseButtonActions(t *testing.T) {
	listed := strings.Split(action.ModeActionNamesString(), ", ")

	inList := make(map[string]bool, len(listed))
	for _, name := range listed {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}

		if inList[trimmed] {
			t.Errorf("action %q is listed more than once", trimmed)
		}

		inList[trimmed] = true
	}

	if len(inList) == 0 {
		t.Fatal("ModeActionNamesString() listed no actions")
	}

	for _, name := range action.KnownNames() {
		actionType, err := name.ToType()
		wantListed := err == nil && actionType.IsMouseButton()

		if gotListed := inList[string(name)]; gotListed != wantListed {
			t.Errorf("action %q listed = %t, want %t (IsMouseButton = %t)",
				name, gotListed, wantListed, wantListed)
		}
	}

	// Everything listed must be a real known name, not an invented string.
	for name := range inList {
		if !action.IsKnownName(action.Name(name)) {
			t.Errorf("ModeActionNamesString() listed %q, which is not a known action name", name)
		}
	}
}

// TestPrimaryModifier_MatchesThePlatformAccelerator pins the platform's default
// accelerator. Getting it backwards would make every default binding fire on
// the wrong modifier — Control on macOS, Command on Linux and Windows.
func TestPrimaryModifier_MatchesThePlatformAccelerator(t *testing.T) {
	want := action.ModCtrl
	if runtime.GOOS == "darwin" {
		want = action.ModCmd
	}

	if got := action.PrimaryModifier(); got != want {
		t.Errorf("PrimaryModifier() on %s = %v, want %v", runtime.GOOS, got, want)
	}

	// The two candidates must be distinct, or the assertion above proves
	// nothing on either platform.
	if action.ModCmd == action.ModCtrl {
		t.Fatal("ModCmd and ModCtrl are equal; the platform assertion is vacuous")
	}
}
