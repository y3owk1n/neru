package action_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
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
		{action.NameRefreshHints, action.IsRefreshHintsAction},
		{action.NameSnap, action.IsSnapAction},
		{action.NameFeed, action.IsFeedAction},
	}
}

// TestActionPredicates_MatchExactlyTheirOwnName runs every predicate against
// every known action name plus a few near-misses, and requires each to accept
// its own name and reject all others.
//
// The Is<X>Action helpers are the routing table for CLI and IPC action
// dispatch. Each one claims a single action name, and the caller runs whichever
// branch says yes. They are one-liners, which is why they went untested and why
// a slip in one goes unnoticed. An inverted comparison makes a predicate answer
// "yes" for every action except its own, and every unrelated action then takes
// that branch.
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

// TestModeActionNamesString_ListsExactlyTheModeActions checks the help text
// against what a mode activation accepts. If the two drift, `neru mode
// --help` advertises actions that are then rejected, or lists ones a mode
// refuses.
func TestModeActionNamesString_ListsExactlyTheModeActions(t *testing.T) {
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

	for name := range inList {
		_, err := modecmd.Parse(domain.ModeHints, []string{"--action=" + name})
		if err != nil {
			t.Errorf("ModeActionNamesString() lists %q, but a mode refuses it: %v", name, err)
		}
	}

	// Every known name a mode accepts must be in the help. Known names a mode
	// refuses, such as scroll, fail the loop above if listed.
	for _, name := range action.KnownNames() {
		_, err := modecmd.Parse(domain.ModeHints, []string{"--action=" + string(name)})
		if err == nil && !inList[string(name)] {
			t.Errorf("ModeActionNamesString() omits %q, which a mode accepts", name)
		}
	}
}

// TestPrimaryModifier_MatchesThePlatformAccelerator pins the platform's default
// accelerator. Getting it backwards would make every default binding fire on
// the wrong modifier, Control on macOS and Command on Linux and Windows.
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
