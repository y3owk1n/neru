//go:build linux

package atspi

import (
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
)

const (
	appIDFirefox     = "org.mozilla.firefox"
	appKonsole       = "konsole"
	appKonsoleDotted = "org.kde.konsole"
	appFirefox       = "firefox"
	titleEditor      = "Editor"
)

func TestAppMatchesFocusedID(t *testing.T) {
	cases := []struct {
		name      string
		atspiName string
		appID     string
		want      bool
	}{
		{"reverse-dns app_id vs display name", "Firefox", appIDFirefox, true},
		{"bare app_id case-insensitive", "Helium", "helium", true},
		{"exact match", appKonsole, appKonsole, true},
		{"reverse-dns both sides", appKonsoleDotted, appKonsole, true},
		{"kde reverse-dns app_id", "Konsole", appKonsoleDotted, true},
		{"no match", "Unnamed", appIDFirefox, false},
		{"empty atspi name", "", appIDFirefox, false},
		{"empty app_id", "Firefox", "", false},
		{"segment does not collide", "fox", appIDFirefox, false},
		{"different apps", "Nautilus", "org.gnome.Calculator", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := appMatchesFocusedID(tc.atspiName, tc.appID); got != tc.want {
				t.Errorf("appMatchesFocusedID(%q, %q) = %v, want %v",
					tc.atspiName, tc.appID, got, tc.want)
			}
		})
	}
}

func TestLastDotSegment(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"org.mozilla.firefox", appFirefox},
		{"helium", ""},
		{"trailing.", ""},
		{"", ""},
		{"a.b", "b"},
		{".leading", "leading"},
	}

	for _, tc := range cases {
		if got := lastDotSegment(tc.in); got != tc.want {
			t.Errorf("lastDotSegment(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTitleMatchesFocused(t *testing.T) {
	cases := []struct {
		name         string
		frameTitle   string
		focusedTitle string
		want         bool
	}{
		{"exact match", "Google — Mozilla Firefox", "Google — Mozilla Firefox", true},
		{"whitespace trimmed", "  " + titleEditor + "  ", titleEditor, true},
		{"different windows", "Tab A — Firefox", "Tab B — Firefox", false},
		{"empty frame title never matches", "", "", false},
		{"empty focused title", titleEditor, "", false},
		{"empty frame with non-empty focused", "", titleEditor, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := titleMatchesFocused(tc.frameTitle, tc.focusedTitle); got != tc.want {
				t.Errorf("titleMatchesFocused(%q, %q) = %v, want %v",
					tc.frameTitle, tc.focusedTitle, got, tc.want)
			}
		})
	}
}

func TestSelectFrame(t *testing.T) {
	// Distinct refs so the test can tell which candidate was chosen.
	var (
		fTitle  = accRef{Name: "focused-title"}
		fShow   = accRef{Name: "focused-showing"}
		active  = accRef{Name: "active"}
		activeA = accRef{Name: "active-any"}
		showing = accRef{Name: "showing"}
		shell   = accRef{Name: "shell"}
	)

	cases := []struct {
		name   string
		cand   frameCandidates
		want   accRef
		wantOK bool
	}{
		{
			name: "unique title match wins even with sibling windows",
			cand: frameCandidates{
				focusedTitleFrame: fTitle, focusedTitleCount: 1,
				focusedShowingFrame: fShow, focusedShowingCount: 3,
				activeShowing: active, haveActiveShowing: true,
				haveFocused: true,
			},
			want: fTitle, wantOK: true,
		},
		{
			name: "single showing window is unambiguous without a title match",
			cand: frameCandidates{
				focusedShowingFrame: fShow, focusedShowingCount: 1,
				activeShowing: active, haveActiveShowing: true,
				haveFocused: true,
			},
			want: fShow, wantOK: true,
		},
		{
			name: "ambiguous title (duplicate siblings) on KDE uses ACTIVE",
			cand: frameCandidates{
				focusedTitleFrame: fTitle, focusedTitleCount: 2,
				focusedShowingFrame: fShow, focusedShowingCount: 2,
				activeShowing: active, haveActiveShowing: true,
				haveFocused: true, activeStateIdentifiesFocus: true,
			},
			want: active, wantOK: true,
		},
		{
			name: "ambiguous title on wlroots returns nothing (no sibling guess)",
			cand: frameCandidates{
				focusedTitleFrame: fTitle, focusedTitleCount: 2,
				focusedShowingFrame: fShow, focusedShowingCount: 2,
				activeShowing: active, haveActiveShowing: true,
				haveFocused: true, activeStateIdentifiesFocus: false,
			},
			want: accRef{}, wantOK: false,
		},
		{
			name: "multi-window no title match on wlroots returns nothing",
			cand: frameCandidates{
				focusedShowingFrame: fShow, focusedShowingCount: 2,
				activeShowing: active, haveActiveShowing: true,
				showingAny: showing, haveShowingAny: true,
				shellShowing: shell, haveShell: true,
				haveFocused: true, activeStateIdentifiesFocus: false,
			},
			want: accRef{}, wantOK: false,
		},
		{
			name: "name-mismatched app (no focused frame) on KDE uses ACTIVE",
			cand: frameCandidates{
				activeShowing: active, haveActiveShowing: true,
				showingAny: showing, haveShowingAny: true,
				shellShowing: shell, haveShell: true,
				haveFocused: true, activeStateIdentifiesFocus: true,
			},
			want: active, wantOK: true,
		},
		{
			name: "KDE unmatched focus with no ACTIVE returns nothing (not showing/shell)",
			cand: frameCandidates{
				showingAny: showing, haveShowingAny: true,
				shellShowing: shell, haveShell: true,
				haveFocused: true, activeStateIdentifiesFocus: true,
			},
			want: accRef{}, wantOK: false,
		},
		{
			name: "no focused app_id (X11/GNOME) falls back to active+showing",
			cand: frameCandidates{
				activeShowing: active, haveActiveShowing: true,
				activeAny: activeA, haveActiveAny: true,
				showingAny: showing, haveShowingAny: true,
			},
			want: active, wantOK: true,
		},
		{
			name: "no focus, only showing available",
			cand: frameCandidates{
				showingAny: showing, haveShowingAny: true,
				shellShowing: shell, haveShell: true,
			},
			want: showing, wantOK: true,
		},
		{
			name: "no focus, only shell available (last resort)",
			cand: frameCandidates{shellShowing: shell, haveShell: true},
			want: shell, wantOK: true,
		},
		{
			name:   "nothing found",
			cand:   frameCandidates{},
			want:   accRef{},
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := selectFrame(tc.cand)
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("selectFrame() = (%+v, %v), want (%+v, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestStateBitset(t *testing.T) {
	got := stateBitset(atspiStateShowing) // SHOWING = 25
	if len(got) != atspiStateWords {
		t.Fatalf("len = %d, want %d", len(got), atspiStateWords)
	}

	if got[0] != 1<<25 || got[1] != 0 {
		t.Fatalf("stateBitset(SHOWING) = %v, want [%d 0]", got, int32(1)<<25)
	}
}

func TestRoleBitfield(t *testing.T) {
	// push button 43 -> word1 bit11; link 88 -> word2 bit24; menu 129 -> word4 bit1.
	got := roleBitfield([]int32{43, 88, 129})
	if len(got) != atspiRoleWords {
		t.Fatalf("len = %d, want %d", len(got), atspiRoleWords)
	}

	want := make([]int32, atspiRoleWords)
	for _, id := range []int32{43, 88, 129} {
		want[id/32] |= 1 << uint(id%32)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("word %d = %d, want %d (got=%v want=%v)", i, got[i], want[i], got, want)
		}
	}

	// Explicit spot-checks guard against off-by-one word/bit packing.
	if got[1] != 1<<11 {
		t.Errorf("word1 = %d, want %d (push button)", got[1], int32(1)<<11)
	}

	if got[4] != 1<<1 {
		t.Errorf("word4 = %d, want %d (push-button-menu)", got[4], int32(1)<<1)
	}
}

func TestDeriveTargetRoleIDs(t *testing.T) {
	noDuplicates := func(t *testing.T, ids []int32) {
		t.Helper()

		seen := make(map[int32]bool)
		for _, roleID := range ids {
			if seen[roleID] {
				t.Errorf("duplicate id %d in %v", roleID, ids)
			}

			seen[roleID] = true
		}
	}

	t.Run("the button semantic role expands to its atspi ids (deduped)", func(t *testing.T) {
		// push button(43), button(43 -> deduped), toggle button(62).
		ids := deriveTargetRoleIDs(rolesSet(resolveLinuxRoles(t, "button")))
		for _, want := range []int32{43, 62} {
			if !slices.Contains(ids, want) {
				t.Errorf("missing role id %d in %v", want, ids)
			}
		}

		noDuplicates(t, ids)

		if len(ids) != 2 {
			t.Errorf("button -> %v, want exactly ids {43, 62}", ids)
		}
	})

	t.Run("checkbox gathers check box and check menu item", func(t *testing.T) {
		ids := deriveTargetRoleIDs(rolesSet(resolveLinuxRoles(t, "checkbox")))
		for _, want := range []int32{7, 8} {
			if !slices.Contains(ids, want) {
				t.Errorf("missing role id %d in %v", want, ids)
			}
		}

		noDuplicates(t, ids)
	})

	t.Run("text_field gathers entry and password text", func(t *testing.T) {
		ids := deriveTargetRoleIDs(rolesSet(resolveLinuxRoles(t, "text_field")))
		for _, want := range []int32{79, 40} {
			if !slices.Contains(ids, want) {
				t.Errorf("missing role id %d in %v", want, ids)
			}
		}
	})

	t.Run("multiple semantic roles union their ids", func(t *testing.T) {
		ids := deriveTargetRoleIDs(rolesSet(resolveLinuxRoles(t, "list_item", "row", "button")))
		for _, want := range []int32{32, 90, 43, 62} { // list item, table row, push/toggle button
			if !slices.Contains(ids, want) {
				t.Errorf("missing role id %d in %v", want, ids)
			}
		}

		noDuplicates(t, ids)
	})

	t.Run("raw AT-SPI names resolve on the fast path", func(t *testing.T) {
		// A native role addressed through the atspi: prefix must reach the
		// Collection query rather than silently downgrading to the walk.
		ids := deriveTargetRoleIDs(rolesSet(resolveLinuxRoles(t, "atspi:spin button")))
		if !slices.Contains(ids, 52) {
			t.Errorf("spin button -> %v, want id 52", ids)
		}
	})

	t.Run("role with no atspi equivalent yields none", func(t *testing.T) {
		ids := deriveTargetRoleIDs(map[string]struct{}{"not a real role": {}})
		if len(ids) != 0 {
			t.Errorf("unknown role -> %v, want none", ids)
		}
	})
}

// resolveLinuxRoles turns semantic or prefixed config entries into the native
// AT-SPI role names the client works with, the same way config load does.
func resolveLinuxRoles(t *testing.T, entries ...string) []string {
	t.Helper()

	resolution := element.ResolveRoles(entries, "linux")
	if resolution.HasFatal() {
		t.Fatalf("resolving %v: %v", entries, resolution.FatalMessages())
	}

	return resolution.Native
}

// TestAtspiRoleNameToIDCoversTheVocabulary pins the Collection fast path
// against the semantic vocabulary. Every AT-SPI role name the vocabulary can
// expand to must have an AtspiRole id, otherwise that role would be found by
// the walk but silently missed by Collection.GetMatches.
func TestAtspiRoleNameToIDCoversTheVocabulary(t *testing.T) {
	for _, mapping := range element.RoleVocabulary {
		for _, native := range mapping.ATSPI {
			if _, ok := atspiRoleNameToID[native]; !ok {
				t.Errorf(
					"semantic role %q expands to AT-SPI role %q but atspiRoleNameToID "+
						"lacks it; Collection.GetMatches would not match that role",
					mapping.Semantic, native,
				)
			}
		}
	}
}

// TestAtspiRoleNames_CoverTheEnum checks the role list against the shape of the
// AtspiRole enum. Ids are the slice indices, so gaps and duplicates cannot
// occur by construction; what can still go wrong is a name being added or
// dropped, which shifts every id after it. Pinning the length and the last
// entry catches that, and TestAtspiRoleNameToIDAnchors catches a reordering.
// ATSPI_ROLE_LAST_DEFINED (131) is a sentinel and must not be listed.
func TestAtspiRoleNames_CoverTheEnum(t *testing.T) {
	const lastRoleID = 130 // ATSPI_ROLE_SWITCH

	if len(atspiRoleNames) != lastRoleID+1 {
		t.Errorf(
			"atspiRoleNames has %d entries, want %d (ids 0..%d)",
			len(atspiRoleNames), lastRoleID+1, lastRoleID,
		)
	}

	if last := atspiRoleNames[len(atspiRoleNames)-1]; last != "switch" {
		t.Errorf("last role name = %q, want \"switch\" (ATSPI_ROLE_SWITCH)", last)
	}

	seen := make(map[string]int, len(atspiRoleNames))

	for index, name := range atspiRoleNames {
		if previous, duplicate := seen[name]; duplicate {
			t.Errorf("role name %q appears at both %d and %d", name, previous, index)
		}

		seen[name] = index
	}

	// Every alias must point at a name that exists, or it silently resolves to
	// nothing and the spelling it exists to accept stops working.
	for alias, canonical := range atspiRoleNameAliases {
		if _, ok := seen[canonical]; !ok {
			t.Errorf("alias %q points at unknown role name %q", alias, canonical)
		}

		if _, ok := atspiRoleNameToID[alias]; !ok {
			t.Errorf("alias %q is missing from atspiRoleNameToID", alias)
		}
	}
}

// TestAtspiRoleNameToIDAnchors verifies the reconstructed AtspiRole enum
// against ids that were independently known before the table was completed. A
// mistake in the enum ordering would shift every id after the error, so these
// anchors, spread across the enum, catch a bad transcription.
func TestAtspiRoleNameToIDAnchors(t *testing.T) {
	anchors := map[string]int32{
		"check box":             7,
		"check menu item":       8,
		"combo box":             11,
		"list item":             32,
		"menu item":             35,
		"page tab":              37,
		"password text":         40,
		atspiRolePushButton:     43,
		"radio button":          44,
		"radio menu item":       45,
		"slider":                51,
		"table cell":            56,
		"toggle button":         62,
		"entry":                 79,
		"link":                  88,
		"table row":             90,
		atspiRolePushButtonMenu: 129,
	}

	for name, want := range anchors {
		got, ok := atspiRoleNameToID[name]
		if !ok {
			t.Errorf("atspiRoleNameToID lacks anchor role %q", name)

			continue
		}

		if got != want {
			t.Errorf("atspiRoleNameToID[%q] = %d, want %d", name, got, want)
		}
	}
}
