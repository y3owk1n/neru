//go:build integration && linux && cgo

package linux

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/ports"
)

// missingFamily is a family name no machine has installed. It has to be absent
// for the fallback case to mean anything, so the test verifies that rather than
// trusting it.
const missingFamily = "Neru No Such Font Family 4f2a"

func TestFontconfigResolver_Resolve_GenericAliasesGiveAnInstalledFamily(t *testing.T) {
	// Which spellings count as generic is pinned by the shared parser
	// (platform/fontgeneric); what matters here is that each one resolves to a
	// family this machine has.
	aliases := []string{
		"", "sans", "Sans Serif", "sans-serif", "sans_serif",
		"serif", "mono", "monospace",
	}

	installed := installedFamilies(t)
	resolver := NewFontResolver()

	for _, alias := range aliases {
		name := alias
		if name == "" {
			name = "empty"
		}

		t.Run(name, func(t *testing.T) {
			got := resolver.Resolve(alias)

			if !isInstalled(installed, got) {
				t.Fatalf(
					"Resolve(%q) = %q, which no installed family matches; a "+
						"generic alias must resolve to a family this machine has",
					alias,
					got,
				)
			}
		})
	}
}

func TestFontconfigResolver_Resolve_InstalledFamilyComesBackAsWritten(t *testing.T) {
	installed := installedFamilies(t)
	family := aFamilyWorthRespelling(t, installed)
	resolver := NewFontResolver()

	// The respellings are the cases that separate the two rules: fontconfig
	// matches them all (its family comparison ignores case and blanks) and
	// reports the family's own capitalisation, where the port promises back the
	// name the caller wrote.
	cases := map[string]string{
		family:                              family,
		"  " + family + "  ":                family,
		strings.ToLower(family):             strings.ToLower(family),
		strings.ReplaceAll(family, " ", ""): strings.ReplaceAll(family, " ", ""),
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := resolver.Resolve(input); got != want {
				t.Fatalf("Resolve(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestFontconfigResolver_Resolve_MissingFamilyFallsBackToTheResolvedGeneric(t *testing.T) {
	installed := installedFamilies(t)
	if isInstalled(installed, missingFamily) {
		t.Skipf("%q is installed here; the fallback cannot be exercised", missingFamily)
	}

	// Names a stock fontconfig carries a substitution rule for. On a machine
	// that has none of them they are the sharp version of the missing-family
	// case: fontconfig answers each with a different family's name, so
	// reporting what fontconfig matched and falling back to the generic are
	// visibly different answers.
	substituted := []string{"Helvetica", "Arial", "Times New Roman", "Courier New"}

	resolver := NewFontResolver()
	want := theResolvedGeneric(t, installed, resolver)

	missing := []string{missingFamily}

	for _, family := range substituted {
		if !isInstalled(installed, family) {
			missing = append(missing, family)
		}
	}

	for _, family := range missing {
		t.Run(family, func(t *testing.T) {
			got := resolver.Resolve(family)

			if got != want {
				t.Fatalf(
					"Resolve(%q) = %q, want the resolved generic %q — a family "+
						"that is not installed falls back to the generic, not "+
						"to whatever fontconfig would substitute for it",
					family,
					got,
					want,
				)
			}

			if !isInstalled(installed, got) {
				t.Fatalf("Resolve(%q) = %q, which no installed family matches", family, got)
			}
		})
	}
}

// theResolvedGeneric is the family a missing one falls back to: the Linux sans
// baseline on any machine that has it, which is the answer stated outright
// rather than read back out of the resolver. Only where the baseline itself is
// missing does it have to ask, since what fontconfig substitutes there is the
// machine's own business.
func theResolvedGeneric(t *testing.T, installed []string, resolver ports.FontResolver) string {
	t.Helper()

	if isInstalled(installed, defaultLinuxSans) {
		return defaultLinuxSans
	}

	return resolver.Resolve("sans")
}

// installedFamilies asks fontconfig which families this machine has, through
// fc-list rather than through the resolver: every case here needs to know what
// is installed, and the code under test cannot be the one to say. Skips when
// fontconfig has nothing to report, which is a machine the cases cannot run on
// rather than a failure.
func installedFamilies(t *testing.T) []string {
	t.Helper()

	_, lookErr := exec.LookPath("fc-list")
	if lookErr != nil {
		t.Skipf("fc-list is not installed, so what fontconfig has cannot be read: %v", lookErr)
	}

	listed, listErr := exec.CommandContext(
		t.Context(), "fc-list", "--format", "%{family}\n",
	).Output()
	if listErr != nil {
		t.Fatalf("fc-list error = %v", listErr)
	}

	var families []string

	for line := range strings.SplitSeq(string(listed), "\n") {
		// A font carrying several family names is printed as one
		// comma-separated line; each of them is installed.
		for family := range strings.SplitSeq(line, ",") {
			if trimmed := strings.TrimSpace(family); trimmed != "" {
				families = append(families, trimmed)
			}
		}
	}

	if len(families) == 0 {
		t.Skip("no fonts are installed on this machine")
	}

	return families
}

// aFamilyWorthRespelling returns an installed family that can be asked for in a
// spelling other than its own — one with capitals to lower and a space to
// remove, so the respellings are different strings from the name fontconfig
// holds.
func aFamilyWorthRespelling(t *testing.T, installed []string) string {
	t.Helper()

	for _, family := range installed {
		if family != strings.ToLower(family) && strings.Contains(family, " ") {
			return family
		}
	}

	t.Skip("no installed family has both capitals and a space to respell")

	return ""
}

// isInstalled reports whether any installed family is the given name, compared
// the way fontconfig compares family names: ignoring case and blanks.
func isInstalled(installed []string, family string) bool {
	for _, candidate := range installed {
		if sameFamily(candidate, family) {
			return true
		}
	}

	return false
}

// sameFamily compares two family names ignoring case and blanks, which is what
// fontconfig's own family comparison does.
func sameFamily(a, b string) bool {
	fold := func(name string) string {
		return strings.ToLower(strings.ReplaceAll(name, " ", ""))
	}

	return fold(a) == fold(b)
}
