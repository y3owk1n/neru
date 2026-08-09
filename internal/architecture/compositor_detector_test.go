package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// compositorIdentityEnv are the environment variables that answer "which
// desktop or compositor family is this session". Reading one is how a caller
// forms its own opinion about the compositor, which is what
// internal/adapter/platform/AGENTS.md forbids outside the detector.
//
// Four of them are read by nothing today and are here anyway: the point of the
// list is that the *next* detector cannot be written either, and these are the
// variables a contributor would reach for.
//
// Deliberately not in this set, because neither asks this question:
//
//   - WAYLAND_DISPLAY, DISPLAY — socket and display names. Code reads them to
//     find out whether there is something to connect to, which is a
//     precondition on a connection rather than an opinion about the compositor.
//     WAYLAND_DISPLAY is one of the detector's own inputs, so a reader outside
//     it can look like a rival; it is not one, because "is the Wayland socket
//     exported" and "which compositor family is this" have different answers in
//     exactly the session that motivated this guardrail.
//   - SWAYSOCK, NIRI_SOCKET, HYPRLAND_INSTANCE_SIGNATURE — the wlroots socket
//     trio, which asks which compositor's CLI to shell out to. LinuxBackend has
//     one value covering niri, sway, Hyprland, River and Wayfire, so it cannot
//     answer that and folding the trio in would lose the answer. Nothing here
//     confines them; #1430 is the ticket for the contract they need.
var compositorIdentityEnv = []string{
	"XDG_CURRENT_DESKTOP",
	"XDG_SESSION_TYPE",
	"XDG_SESSION_DESKTOP",
	"DESKTOP_SESSION",
	"GDMSESSION",
}

// compositorDetectorFile is the file internal/adapter/platform/AGENTS.md names
// as the one place the live compositor is detected.
const compositorDetectorFile = "internal/adapter/platform/backend_linux.go"

// compositorIdentityEchoes maps a file that names one of these variables
// without deciding anything from it to why it is allowed to.
//
// TestCompositorIdentityEchoesStayHonest fails on an entry whose file has
// stopped reading one, so this list can only shrink.
var compositorIdentityEchoes = map[string]string{
	"internal/adapter/platform/factory_messages_linux.go": "quotes the value back " +
		"to the user in the unsupported-compositor error, after the detector has " +
		"already read it and decided; it branches on nothing",
}

// TestCompositorFamilyIsDecidedInOnePlace pins
// internal/adapter/platform/AGENTS.md: backend_linux.go detects the live
// compositor, and nothing else probes for it.
//
// The rule earned a guardrail by being broken silently. DetectLinuxDisplayServer
// read XDG_SESSION_TYPE three lines from the canonical detector's own call site
// and applied its own precedence, so a sway or Hyprland session launched from a
// systemd user unit that imported SWAYSOCK but not WAYLAND_DISPLAY ran the X11
// adapter while `neru info` and the health output reported
// display_server: wayland. Everything compiled, linted and tested green.
//
// It judges non-test Go files. A test that sets one of these variables is
// driving the detector through its environment, which is the detector being
// exercised rather than a second opinion about the session.
func TestCompositorFamilyIsDecidedInOnePlace(t *testing.T) {
	reads := compositorIdentityReads(t)

	if len(reads[compositorDetectorFile]) == 0 {
		t.Fatalf(
			"%s reads none of %v; either the detector moved and this guardrail "+
				"now names the wrong file, or the match is broken and every check "+
				"below would pass vacuously",
			compositorDetectorFile, compositorIdentityEnv,
		)
	}

	for file, variables := range reads {
		if file == compositorDetectorFile {
			continue
		}

		if _, echoes := compositorIdentityEchoes[file]; echoes {
			continue
		}

		t.Errorf(
			"%s reads %s; the compositor family is decided in %s and nowhere else, "+
				"so ask platform.DetectLinuxBackend for it "+
				"(internal/adapter/platform/AGENTS.md: never probe the compositor elsewhere)",
			file, strings.Join(variables, ", "), compositorDetectorFile,
		)
	}
}

// TestCompositorIdentityEchoesStayHonest keeps an exemption from outliving what
// it describes: an entry naming a file that no longer reads one of these
// variables is a license nobody needs, sitting ready for a real probe to move
// in under it.
func TestCompositorIdentityEchoesStayHonest(t *testing.T) {
	reads := compositorIdentityReads(t)

	for file, reason := range compositorIdentityEchoes {
		if len(reads[file]) > 0 {
			continue
		}

		t.Errorf(
			"compositorIdentityEchoes names %s, which reads none of %v; drop the "+
				"entry (%s)",
			file, compositorIdentityEnv, reason,
		)
	}
}

// compositorIdentityReads returns, per non-test Go file, the compositor-identity
// variables it names as a string literal — which is how every read of one is
// spelled, whether through os.Getenv, os.LookupEnv or a lookup of its own.
//
// Equality, not containment: factory_messages_linux.go's error text embeds
// "XDG_CURRENT_DESKTOP=%q" beside its read, and a message naming the variable
// it is reporting on is not a probe.
func compositorIdentityReads(t *testing.T) map[string][]string {
	t.Helper()

	repoRoot := findRepoRoot(t)
	fileSet := token.NewFileSet()
	reads := map[string][]string{}
	checked := 0

	walkRepoFiles(t, repoRoot, func(file repoFile) {
		if filepath.Ext(file.name) != goExt || strings.HasSuffix(file.name, "_test.go") {
			return
		}

		parsed, parseErr := parser.ParseFile(fileSet, file.abs, nil, 0)
		if parseErr != nil {
			t.Fatalf("ParseFile(%s) error = %v", file.rel, parseErr)
		}

		checked++

		ast.Inspect(parsed, func(node ast.Node) bool {
			literal, isLiteral := node.(*ast.BasicLit)
			if !isLiteral || literal.Kind != token.STRING {
				return true
			}

			value, unquoteErr := strconv.Unquote(literal.Value)
			if unquoteErr != nil || !slices.Contains(compositorIdentityEnv, value) {
				return true
			}

			if !slices.Contains(reads[file.rel], value) {
				reads[file.rel] = append(reads[file.rel], value)
			}

			return true
		})
	})

	assertWalkedAtLeast(t, "Go files that could probe the compositor", checked, bulkWalkFloor)

	for _, variables := range reads {
		slices.Sort(variables)
	}

	return reads
}
