package architecture_test

import (
	"slices"
	"strings"
	"testing"
)

// compositorSocketEnv are the environment variables a wlroots-family compositor
// exports to point at its own IPC endpoint. Reading one answers "which
// compositor's CLI do I shell out to" — niri, Sway or Hyprland — which
// platform.LinuxBackend deliberately cannot answer, because one value covers
// those three plus River and Wayfire.
//
// That makes them the opposite case to the identity variables in
// compositor_detector_test.go, and the two sets are disjoint on purpose. An
// identity variable is confined to the detector, because deciding the
// compositor family twice is what produced two disagreeing answers. A socket
// variable stays readable outside it — but only downstream of the decision,
// never as a way to make one. The socket says which compositor is *reachable*,
// not which one the session is running under: a systemd user unit that imported
// SWAYSOCK into an X11 login leaves it set on a session with no Sway in sight.
var compositorSocketEnv = []string{
	"HYPRLAND_INSTANCE_SIGNATURE",
	"NIRI_SOCKET",
	"SWAYSOCK",
}

// compositorSocketReaders maps each file allowed to read one of these variables
// to the gate that already decided the session is wlroots before it does.
//
// TestCompositorSocketReadersStayHonest fails on an entry whose file has
// stopped reading one, so this list can only shrink.
var compositorSocketReaders = map[string]string{
	"internal/adapter/platform/linux/system_focused_window.go": "picks the " +
		"compositor CLI to query for focused-window bounds, reached only from " +
		"waylandFocusedWindowSource once the backend has already answered — KDE " +
		"is routed to its KWin bridge before any socket is read, so a KDE " +
		"session that inherited SWAYSOCK is no longer asked about a sway tree",
	"internal/adapter/accessibility/atspi/window_origin.go": "picks the " +
		"compositor CLI to query for the focused window's origin, reached only " +
		"from newWindowOriginSource's BackendWaylandWlroots arm",
	"internal/adapter/platform/linux/system_cursor_ipc.go": "picks the " +
		"compositor CLI to query for the physical cursor position, reached only " +
		"from SystemAdapter.SyncCursorPosition behind waylandUsesWlrClientStack; " +
		"an unset or stale socket answers not-found and the sync falls back to " +
		"layer-shell discovery",
}

// TestCompositorSocketsAreReadOnlyBehindTheBackend pins the second half of
// internal/adapter/platform/AGENTS.md's detection rule: the compositor is
// identified by platform.DetectLinuxBackend, and a compositor socket is a
// detail read after that answer, never a substitute for asking.
//
// The rule earned a guardrail by being broken silently, with a side effect on
// disk. The AT-SPI client chose its window-origin source from these three
// variables alone and defaulted to the KWin bridge when none was set, so a
// plain X11 session — which sets none of them — started the bridge: session
// bus, exported object, bus name, and a KWin script written into
// $XDG_RUNTIME_DIR for a compositor that was not running (#1430). Everything
// compiled, linted and tested green, and the comment above the fallback called
// it a harmless no-op, which was true of the origin lookup and false of the
// start path.
//
// It judges non-test Go files. A test that sets one of these variables is
// exercising the selection it drives, not forming an opinion about the session.
func TestCompositorSocketsAreReadOnlyBehindTheBackend(t *testing.T) {
	reads := envNameReads(t, compositorSocketEnv)

	for file, variables := range reads {
		if _, allowed := compositorSocketReaders[file]; allowed {
			continue
		}

		t.Errorf(
			"%s reads %s; a compositor socket says which compositor CLI is "+
				"reachable, not which compositor this session runs, so ask "+
				"platform.DetectLinuxBackend for the session and read the socket "+
				"only behind that answer (internal/adapter/platform/AGENTS.md: "+
				"never probe the compositor elsewhere)",
			file, strings.Join(variables, ", "),
		)
	}
}

// TestCompositorSocketReadersStayHonest keeps an entry from outliving what it
// describes, and doubles as this pin's floor: an allowlist naming a file that
// reads none of these variables is a license nobody needs, sitting ready for a
// real probe to move in under it — and if every entry has gone quiet, the match
// has broken and the check above passes over nothing.
func TestCompositorSocketReadersStayHonest(t *testing.T) {
	reads := envNameReads(t, compositorSocketEnv)

	for file, reason := range compositorSocketReaders {
		if len(reads[file]) > 0 {
			continue
		}

		t.Errorf(
			"compositorSocketReaders names %s, which reads none of %v; drop the "+
				"entry (%s)",
			file, compositorSocketEnv, reason,
		)
	}
}

// TestCompositorEnvironmentPinsStayDisjoint keeps the two environment rules in
// this package from growing into each other. A variable in both sets would be
// confined to the detector by one test and licensed outside it by the other,
// and whichever failure a contributor read first would send them the wrong way.
func TestCompositorEnvironmentPinsStayDisjoint(t *testing.T) {
	for _, name := range compositorSocketEnv {
		if slices.Contains(compositorIdentityEnv, name) {
			t.Errorf(
				"%s is in both compositorSocketEnv and compositorIdentityEnv; a "+
					"variable either names the session's compositor family (confined "+
					"to the detector) or names one compositor's socket (read after "+
					"the backend decides), and it cannot be pinned as both",
				name,
			)
		}
	}
}
