//go:build linux

//nolint:testpackage // Exercises the unexported app_id matching helpers directly.
package accessibility

import "testing"

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
