//go:build linux

//nolint:testpackage // Exercises the unexported app_id matching helpers directly.
package accessibility

import (
	"slices"
	"testing"
)

const (
	appIDFirefox = "org.mozilla.firefox"
	appKonsole   = "konsole"
	titleEditor  = "Editor"
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
		{"reverse-dns both sides", "org.kde.konsole", appKonsole, true},
		{"kde reverse-dns app_id", "Konsole", "org.kde.konsole", true},
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
		{"org.mozilla.firefox", "firefox"},
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

	t.Run("AXButton maps to its several atspi roles (deduped)", func(t *testing.T) {
		// push button(43), button(43 -> deduped), toggle button(62).
		ids := deriveTargetRoleIDs(map[string]struct{}{axRoleButton: {}})
		for _, want := range []int32{43, 62} {
			if !slices.Contains(ids, want) {
				t.Errorf("missing role id %d in %v", want, ids)
			}
		}

		noDuplicates(t, ids)

		if len(ids) != 2 {
			t.Errorf("AXButton -> %v, want exactly ids {43, 62}", ids)
		}
	})

	t.Run("AXMenuItem gathers all three menu-item roles", func(t *testing.T) {
		// check menu item(8), radio menu item(45), menu item(35).
		ids := deriveTargetRoleIDs(map[string]struct{}{axRoleMenuItem: {}})
		for _, want := range []int32{8, 45, 35} {
			if !slices.Contains(ids, want) {
				t.Errorf("missing role id %d in %v", want, ids)
			}
		}

		noDuplicates(t, ids)
	})

	t.Run("AXTextField gathers entry and password text", func(t *testing.T) {
		ids := deriveTargetRoleIDs(map[string]struct{}{axRoleTextField: {}})
		for _, want := range []int32{79, 40} {
			if !slices.Contains(ids, want) {
				t.Errorf("missing role id %d in %v", want, ids)
			}
		}
	})

	t.Run("multiple AX roles union their ids", func(t *testing.T) {
		ids := deriveTargetRoleIDs(map[string]struct{}{axRoleRow: {}, axRoleButton: {}})
		for _, want := range []int32{32, 90, 43, 62} { // list item, table row, push/toggle button
			if !slices.Contains(ids, want) {
				t.Errorf("missing role id %d in %v", want, ids)
			}
		}

		noDuplicates(t, ids)
	})

	t.Run("raw AT-SPI names never match (parity with the walk)", func(t *testing.T) {
		// deriveTargetRoleIDs keys on the AX values of atspiToAXRole, exactly like
		// the walk, so a set of raw AT-SPI role names resolves to nothing.
		ids := deriveTargetRoleIDs(map[string]struct{}{"spin button": {}, "toolbar": {}})
		if len(ids) != 0 {
			t.Errorf("atspi-name set -> %v, want none", ids)
		}
	})

	t.Run("role with no atspi equivalent yields none", func(t *testing.T) {
		ids := deriveTargetRoleIDs(map[string]struct{}{"AXUnknownWidget": {}})
		if len(ids) != 0 {
			t.Errorf("unknown role -> %v, want none", ids)
		}
	})
}

func TestAtspiRoleNameToIDCoversAtspiToAXRole(t *testing.T) {
	// Every AT-SPI role name that maps to an AX role (and is therefore hintable
	// via the walk) must also have an AtspiRole id. If the two maps drift, the
	// Collection fast path would silently miss that role type while the walk
	// still finds it — a subtle correctness gap.
	for atspiName := range atspiToAXRole {
		if _, ok := atspiRoleNameToID[atspiName]; !ok {
			t.Errorf("atspiToAXRole has %q but atspiRoleNameToID lacks it; "+
				"Collection.GetMatches would not match that role", atspiName)
		}
	}
}
