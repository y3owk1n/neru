//go:build linux

package linux

import (
	"testing"
)

// The compositor sockets the wlroots arm reads, named once so a case sets the
// same variable the reset loop clears.
const (
	niriSocketEnv        = "NIRI_SOCKET"
	swaySocketEnv        = "SWAYSOCK"
	hyprlandSignatureEnv = "HYPRLAND_INSTANCE_SIGNATURE"
)

// TestWaylandFocusedWindowSource_FollowsTheBackend pins which source answers
// for each session, including the two orderings that were wrong before: KDE
// fell through the socket switch to nothing at all, and a KDE session that
// inherited a stale wlroots socket asked that compositor's CLI about a window
// it has never seen.
func TestWaylandFocusedWindowSource_FollowsTheBackend(t *testing.T) {
	cases := []struct {
		name    string
		backend string
		sockets map[string]string
		want    focusedWindowSource
	}{
		{"kde asks the KWin bridge", backendWaylandKDE, nil, focusedWindowSourceKWin},
		{
			"kde ignores a stale wlroots socket",
			backendWaylandKDE,
			map[string]string{swaySocketEnv: "/run/sway.sock"},
			focusedWindowSourceKWin,
		},
		{
			"wlroots picks niri by its socket",
			backendWaylandWlroots,
			map[string]string{niriSocketEnv: "/run/niri.sock"},
			focusedWindowSourceNiri,
		},
		{
			"wlroots picks sway by its socket",
			backendWaylandWlroots,
			map[string]string{swaySocketEnv: "/run/sway.sock"},
			focusedWindowSourceSway,
		},
		{
			"wlroots picks Hyprland by its signature",
			backendWaylandWlroots,
			map[string]string{hyprlandSignatureEnv: "abc"},
			focusedWindowSourceHyprland,
		},
		{
			"a wlroots compositor with no IPC has no source",
			backendWaylandWlroots,
			nil,
			focusedWindowSourceNone,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, name := range []string{niriSocketEnv, swaySocketEnv, hyprlandSignatureEnv} {
				t.Setenv(name, testCase.sockets[name])
			}

			got := waylandFocusedWindowSource(testCase.backend)
			if got != testCase.want {
				t.Errorf("waylandFocusedWindowSource(%q) = %v, want %v",
					testCase.backend, got, testCase.want)
			}
		})
	}
}
