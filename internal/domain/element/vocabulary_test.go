package element_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// Go platform identifiers used by the table-driven cases below.
const (
	testGOOSDarwin  = "darwin"
	testGOOSLinux   = "linux"
	testGOOSWindows = "windows"

	// testRoleButton is the semantic role most cases resolve.
	testRoleButton = "button"

	// testATSPIPushButton is the AT-SPI role name for role id 43 on
	// at-spi2-core releases predating the ATSPI_ROLE_BUTTON rename.
	testATSPIPushButton = "push button"

	// testATSPIToggleButton is the AT-SPI role older toolkits report for a
	// switch, and one of the roles the button semantic role covers.
	testATSPIToggleButton = "toggle button"

	// Entries reused across the resolution cases.
	testAXButton      = "AXButton"
	testATSPIPageTabs = "atspi:page tab list"
	testTypoRole      = "buton"
)

// TestRoleVocabulary_SemanticNamesAreUnique guards the closed vocabulary: a
// duplicate semantic name would make one of the two mappings unreachable.
func TestRoleVocabulary_SemanticNamesAreUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[element.SemanticRole]struct{}, len(element.RoleVocabulary))

	for _, mapping := range element.RoleVocabulary {
		if _, duplicate := seen[mapping.Semantic]; duplicate {
			t.Errorf("duplicate semantic role %q in element.RoleVocabulary", mapping.Semantic)
		}

		seen[mapping.Semantic] = struct{}{}
	}
}

// TestRoleVocabulary_SemanticNamesDoNotCollideWithDifferentNativeMeanings is
// the guard that keeps the vocabulary readable. A semantic name that is also a
// native role name is fine when both mean the same thing ("button" is both a
// semantic role and an AT-SPI role). It is a trap when they diverge: AT-SPI
// "text" is a text-bearing element, so a semantic role named "text" meaning a
// text field would read as one thing and resolve as another.
func TestRoleVocabulary_SemanticNamesDoNotCollideWithDifferentNativeMeanings(t *testing.T) {
	t.Parallel()

	vocabularies := []element.NativeVocabulary{
		element.VocabularyAX,
		element.VocabularyATSPI,
		element.VocabularyUIA,
	}

	for _, mapping := range element.RoleVocabulary {
		for _, vocab := range vocabularies {
			covering, ok := element.SemanticForNative(vocab, string(mapping.Semantic))
			if !ok || covering == mapping.Semantic {
				continue
			}

			t.Errorf(
				"semantic role %q is also a %s native role name meaning %q; "+
					"rename the semantic role so it cannot be misread",
				mapping.Semantic, vocab, covering,
			)
		}
	}
}

// TestRoleVocabulary_EverySemanticRoleHasAtLeastOneNativeName catches a mapping
// that is empty on every platform, which would be dead vocabulary.
func TestRoleVocabulary_EverySemanticRoleHasAtLeastOneNativeName(t *testing.T) {
	t.Parallel()

	for _, mapping := range element.RoleVocabulary {
		if len(mapping.AX) == 0 && len(mapping.ATSPI) == 0 && len(mapping.UIA) == 0 {
			t.Errorf("semantic role %q maps to no native role on any platform", mapping.Semantic)
		}
	}
}

func TestVocabularyForGOOS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		goos  string
		want  element.NativeVocabulary
		found bool
	}{
		{name: "darwin", goos: testGOOSDarwin, want: element.VocabularyAX, found: true},
		{name: "linux", goos: testGOOSLinux, want: element.VocabularyATSPI, found: true},
		{name: "windows", goos: testGOOSWindows, want: element.VocabularyUIA, found: true},
		{name: "unsupported", goos: "freebsd", want: "", found: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, ok := element.VocabularyForGOOS(testCase.goos)
			if ok != testCase.found {
				t.Fatalf(
					"element.VocabularyForGOOS(%q) found = %v, want %v",
					testCase.goos,
					ok,
					testCase.found,
				)
			}

			if got != testCase.want {
				t.Errorf(
					"element.VocabularyForGOOS(%q) = %q, want %q",
					testCase.goos,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestResolveRoles_SemanticExpansionPerPlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		goos string
		want []string
	}{
		{name: "darwin", goos: testGOOSDarwin, want: []string{testAXButton}},
		{
			name: "linux",
			goos: testGOOSLinux,
			// The AT-SPI role literally named "button" is a distinct native role,
			// not the semantic role of the same spelling.
			want: []string{testATSPIPushButton, testRoleButton, testATSPIToggleButton},
		},
		{name: "windows", goos: testGOOSWindows, want: []string{"Button", "SplitButton"}},
		{name: "unsupported platform resolves to nothing", goos: "freebsd", want: nil},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := element.ResolveRoles([]string{testRoleButton}, testCase.goos)
			if !slices.Equal(got.Native, testCase.want) {
				t.Errorf(
					"element.ResolveRoles(button, %q) = %v, want %v",
					testCase.goos,
					got.Native,
					testCase.want,
				)
			}
		})
	}
}

func TestResolveRoles_PrefixedEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		entry    string
		goos     string
		want     []string
		wantKind element.RoleDiagnosticKind
		wantDiag bool
	}{
		{
			name:  "native entry for this platform is passed through verbatim",
			entry: testATSPIPageTabs,
			goos:  testGOOSLinux,
			want:  []string{"page tab list"},
		},
		{
			name:  "unmapped native name is accepted without complaint",
			entry: "uia:Custom",
			goos:  testGOOSWindows,
			want:  []string{"Custom"},
		},
		{
			name:     "native entry for another platform is ignored, not rejected",
			entry:    "ax:AXDisclosureTriangle",
			goos:     testGOOSLinux,
			wantKind: element.RoleDiagnosticForeignVocabulary,
			wantDiag: true,
		},
		{
			name:     "unknown vocabulary is fatal",
			entry:    "msaa:ROLE_SYSTEM_PUSHBUTTON",
			goos:     testGOOSWindows,
			wantKind: element.RoleDiagnosticUnknownVocabulary,
			wantDiag: true,
		},
		{
			name:     "empty native name is fatal",
			entry:    "atspi:",
			goos:     testGOOSLinux,
			wantKind: element.RoleDiagnosticUnknownVocabulary,
			wantDiag: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := element.ResolveRoles([]string{testCase.entry}, testCase.goos)

			if !slices.Equal(got.Native, testCase.want) {
				t.Errorf(
					"element.ResolveRoles(%q) native = %v, want %v",
					testCase.entry,
					got.Native,
					testCase.want,
				)
			}

			if !testCase.wantDiag {
				if len(got.Diagnostics) != 0 {
					t.Fatalf(
						"element.ResolveRoles(%q) unexpected diagnostics %v",
						testCase.entry,
						got.Diagnostics,
					)
				}

				return
			}

			if len(got.Diagnostics) != 1 {
				t.Fatalf(
					"element.ResolveRoles(%q) diagnostics = %d, want 1",
					testCase.entry, len(got.Diagnostics),
				)
			}

			if got.Diagnostics[0].Kind != testCase.wantKind {
				t.Errorf(
					"element.ResolveRoles(%q) diagnostic kind = %v, want %v",
					testCase.entry, got.Diagnostics[0].Kind, testCase.wantKind,
				)
			}
		})
	}
}

// TestResolveRoles_BareNativeNameSuggestsSemanticRole pins the migration
// message every pre-vocabulary configuration will hit on first load.
func TestResolveRoles_BareNativeNameSuggestsSemanticRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		entry          string
		goos           string
		wantSuggestion string
	}{
		{
			name:           "legacy AX name",
			entry:          testAXButton,
			goos:           testGOOSDarwin,
			wantSuggestion: testRoleButton,
		},
		{
			name:           "legacy AT-SPI name",
			entry:          "push button",
			goos:           testGOOSLinux,
			wantSuggestion: testRoleButton,
		},
		{
			name:           "legacy UIA name",
			entry:          "Edit",
			goos:           testGOOSWindows,
			wantSuggestion: "text_field",
		},
		{
			name:           "role from another platform still suggests the semantic name",
			entry:          "AXSlider",
			goos:           testGOOSLinux,
			wantSuggestion: "slider",
		},
		{
			name:  "genuine typo has no suggestion",
			entry: testTypoRole,
			goos:  testGOOSDarwin,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := element.ResolveRoles([]string{testCase.entry}, testCase.goos)
			if len(got.Diagnostics) != 1 {
				t.Fatalf(
					"element.ResolveRoles(%q) diagnostics = %v, want exactly 1",
					testCase.entry,
					got.Diagnostics,
				)
			}

			diagnostic := got.Diagnostics[0]
			if !diagnostic.IsFatal() {
				t.Errorf("element.ResolveRoles(%q) diagnostic should be fatal", testCase.entry)
			}

			if diagnostic.Kind != element.RoleDiagnosticUnknownName {
				t.Errorf(
					"element.ResolveRoles(%q) kind = %v, want element.RoleDiagnosticUnknownName",
					testCase.entry, diagnostic.Kind,
				)
			}

			if diagnostic.Suggestion != testCase.wantSuggestion {
				t.Errorf(
					"element.ResolveRoles(%q) suggestion = %q, want %q",
					testCase.entry, diagnostic.Suggestion, testCase.wantSuggestion,
				)
			}
		})
	}
}

func TestResolveRoles_DeduplicatesOverlappingExpansions(t *testing.T) {
	t.Parallel()

	// text_field, text_area and search_field all expand onto "Edit" on Windows.
	got := element.ResolveRoles(
		[]string{"text_field", "text_area", "search_field"},
		testGOOSWindows,
	)

	want := []string{"Edit"}
	if !slices.Equal(got.Native, want) {
		t.Errorf("element.ResolveRoles(text roles, windows) = %v, want %v", got.Native, want)
	}

	if got.HasFatal() {
		t.Errorf(
			"element.ResolveRoles(text roles, windows) unexpectedly fatal: %v",
			got.FatalMessages(),
		)
	}
}

func TestResolveRoles_SkipsEmptyEntries(t *testing.T) {
	t.Parallel()

	got := element.ResolveRoles([]string{"", "   ", testRoleButton}, testGOOSDarwin)

	if len(got.Entries) != 1 {
		t.Errorf("element.ResolveRoles() entries = %d, want 1", len(got.Entries))
	}

	if len(got.Diagnostics) != 0 {
		t.Errorf("element.ResolveRoles() diagnostics = %v, want none", got.Diagnostics)
	}
}

func TestResolveRoles_SemanticRoleWithNoPlatformEquivalentIsNotFatal(t *testing.T) {
	t.Parallel()

	got := element.ResolveRoles([]string{"disclosure"}, testGOOSLinux)

	if got.HasFatal() {
		t.Fatalf(
			"element.ResolveRoles(disclosure, linux) should not be fatal: %v",
			got.FatalMessages(),
		)
	}

	if len(got.IgnoredMessages()) != 1 {
		t.Errorf(
			"element.ResolveRoles(disclosure, linux) ignored = %v, want 1",
			got.IgnoredMessages(),
		)
	}
}

// TestResolveRoles_LinuxNamesMatchGetRoleNameOutput pins the AT-SPI expansions
// against what Accessible.GetRoleName actually returns. That name is the GEnum
// nick of the role with hyphens replaced by spaces, so it is not always the
// name a reader would guess: ATSPI_ROLE_PUSH_BUTTON_MENU reports "push button
// menu", never "menu button". A wrong name here matches nothing at runtime
// while still looking plausible in the config.
func TestResolveRoles_LinuxNamesMatchGetRoleNameOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		semantic element.SemanticRole
		want     []string
	}{
		{
			name:     "menu_button uses the push button menu nick",
			semantic: element.SemanticMenuButton,
			want:     []string{"push button menu"},
		},
		{
			name:     "switch covers the dedicated role and the legacy fallback",
			semantic: element.SemanticSwitch,
			want:     []string{"switch", testATSPIToggleButton},
		},
		{
			name:     "button covers both spellings of role id 43",
			semantic: element.SemanticButton,
			want:     []string{testATSPIPushButton, testRoleButton, testATSPIToggleButton},
		},
		{
			name:     "stepper uses the spin button nick",
			semantic: element.SemanticStepper,
			want:     []string{"spin button"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := element.ResolveRoles([]string{string(testCase.semantic)}, testGOOSLinux)
			if !slices.Equal(got.Native, testCase.want) {
				t.Errorf(
					"ResolveRoles(%q, linux) = %v, want %v",
					testCase.semantic, got.Native, testCase.want,
				)
			}
		})
	}
}

// TestDefaultClickableRoles_ResolveOnEverySupportedPlatform is the guard for
// the failure this vocabulary exists to prevent: a default role list that is
// valid but resolves to nothing on some platform, so hints are silently blank
// there. Checking only the platform the tests happen to run on would miss it.
func TestDefaultClickableRoles_ResolveOnEverySupportedPlatform(t *testing.T) {
	t.Parallel()

	for _, goos := range []string{testGOOSDarwin, testGOOSLinux, testGOOSWindows} {
		t.Run(goos, func(t *testing.T) {
			t.Parallel()

			resolution := element.ResolveRoles(element.DefaultClickableRoles, goos)

			if resolution.HasFatal() {
				t.Errorf(
					"DefaultClickableRoles has invalid entries on %s: %v",
					goos, resolution.FatalMessages(),
				)
			}

			if len(resolution.Native) == 0 {
				t.Fatalf(
					"DefaultClickableRoles resolves to no native role on %s; "+
						"hints would be blank out of the box",
					goos,
				)
			}

			// A default that only just resolves is a smell: the shipped list
			// covers buttons, links and text fields on every platform.
			for _, semantic := range []element.SemanticRole{
				element.SemanticButton,
				element.SemanticLink,
				element.SemanticTextField,
			} {
				want := element.ResolveRoles([]string{string(semantic)}, goos).Native
				for _, native := range want {
					if !slices.Contains(resolution.Native, native) {
						t.Errorf(
							"DefaultClickableRoles on %s is missing %q (from %q)",
							goos, native, semantic,
						)
					}
				}
			}
		})
	}
}

// TestResolveRoles_EntriesDescribeEachConfigEntry pins the per-entry record
// that backs `neru roles --explain`. The rendering is only as good as these
// fields, and a wrong Semantic/Vocabulary would mislabel entries in the output
// without failing anything else.
func TestResolveRoles_EntriesDescribeEachConfigEntry(t *testing.T) {
	t.Parallel()

	resolution := element.ResolveRoles(
		[]string{"button", "", "ax:AXGenericElement", testATSPIPageTabs, testTypoRole},
		testGOOSDarwin,
	)

	// The empty entry is skipped silently; the other four are recorded in order.
	if len(resolution.Entries) != 4 {
		t.Fatalf("Entries = %d, want 4 (empty entries skipped): %+v",
			len(resolution.Entries), resolution.Entries)
	}

	semantic := resolution.Entries[0]
	if semantic.Semantic != element.SemanticButton || semantic.Vocabulary != "" {
		t.Errorf("semantic entry = %+v, want Semantic=button with no vocabulary", semantic)
	}

	if len(semantic.Native) == 0 || semantic.Diagnostic != nil {
		t.Errorf("semantic entry = %+v, want native roles and no diagnostic", semantic)
	}

	native := resolution.Entries[1]
	if native.Vocabulary != element.VocabularyAX || native.Semantic != "" {
		t.Errorf("native entry = %+v, want Vocabulary=ax with no semantic", native)
	}

	if !slices.Equal(native.Native, []string{"AXGenericElement"}) {
		t.Errorf("native entry Native = %v, want [AXGenericElement]", native.Native)
	}

	foreign := resolution.Entries[2]
	if foreign.Vocabulary != element.VocabularyATSPI || len(foreign.Native) != 0 {
		t.Errorf("foreign entry = %+v, want Vocabulary=atspi contributing nothing", foreign)
	}

	if foreign.Diagnostic == nil || foreign.Diagnostic.IsFatal() {
		t.Errorf("foreign entry = %+v, want a non-fatal diagnostic", foreign)
	}

	unknown := resolution.Entries[3]
	if unknown.Diagnostic == nil || !unknown.Diagnostic.IsFatal() {
		t.Errorf("unknown entry = %+v, want a fatal diagnostic", unknown)
	}
}

func TestRoleDiagnostic_Message(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		entry    string
		goos     string
		contains []string
	}{
		{
			name:     "unknown name suggests the semantic role",
			entry:    testAXButton,
			goos:     testGOOSDarwin,
			contains: []string{"unknown role", `"` + testAXButton + `"`, `use "button"`},
		},
		{
			name:     "typo has no suggestion to offer",
			entry:    testTypoRole,
			goos:     testGOOSDarwin,
			contains: []string{"unknown role", `"` + testTypoRole + `"`},
		},
		{
			name:     "unknown vocabulary names the accepted prefixes",
			entry:    "msaa:ROLE_SYSTEM_PUSHBUTTON",
			goos:     testGOOSDarwin,
			contains: []string{"unknown role vocabulary", `"ax"`, `"atspi"`, `"uia"`},
		},
		{
			name:     "foreign vocabulary names the platform",
			entry:    testATSPIPageTabs,
			goos:     testGOOSDarwin,
			contains: []string{"does not apply on darwin", "ignored"},
		},
		{
			name:     "semantic role with no equivalent names the platform",
			entry:    "disclosure",
			goos:     testGOOSLinux,
			contains: []string{"no equivalent on linux", "ignored"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			resolution := element.ResolveRoles([]string{testCase.entry}, testCase.goos)
			if len(resolution.Diagnostics) != 1 {
				t.Fatalf("Diagnostics = %v, want exactly 1", resolution.Diagnostics)
			}

			message := resolution.Diagnostics[0].Message()
			for _, want := range testCase.contains {
				if !strings.Contains(message, want) {
					t.Errorf("Message() = %q, want it to contain %q", message, want)
				}
			}
		})
	}
}

func TestResolveRoles_EntryParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		entry string
		goos  string
		want  []string
	}{
		{
			name:  "surrounding whitespace is trimmed",
			entry: "  button  ",
			goos:  testGOOSDarwin,
			want:  []string{testAXButton},
		},
		{
			name:  "whitespace around a prefixed entry is trimmed",
			entry: " atspi :  page tab list ",
			goos:  testGOOSLinux,
			want:  []string{"page tab list"},
		},
		{
			name:  "only the first colon separates the vocabulary",
			entry: "uia:Foo:Bar",
			goos:  testGOOSWindows,
			want:  []string{"Foo:Bar"},
		},
		{
			name:  "native names keep their case",
			entry: "uia:SplitButton",
			goos:  testGOOSWindows,
			want:  []string{"SplitButton"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := element.ResolveRoles([]string{testCase.entry}, testCase.goos)
			if got.HasFatal() {
				t.Fatalf("ResolveRoles(%q) fatal: %v", testCase.entry, got.FatalMessages())
			}

			if !slices.Equal(got.Native, testCase.want) {
				t.Errorf(
					"ResolveRoles(%q) = %v, want %v",
					testCase.entry,
					got.Native,
					testCase.want,
				)
			}
		})
	}
}

// TestResolveRoles_SemanticNamesAreCaseSensitive documents that the semantic
// vocabulary is matched exactly. Accepting "Button" would make the closed set
// fuzzy and weaken typo detection.
func TestResolveRoles_SemanticNamesAreCaseSensitive(t *testing.T) {
	t.Parallel()

	got := element.ResolveRoles([]string{"Button"}, testGOOSDarwin)
	if !got.HasFatal() {
		t.Errorf("ResolveRoles(Button) = %v, want it rejected as unknown", got.Native)
	}
}

// TestAXSubroleNames_PinTheAppKitSubroleDeclarations pins the AX names in the
// vocabulary that AppKit declares as subroles (NSAccessibilitySubrole
// constants), not roles: elements carry them in AXSubrole, so the macOS
// matcher must compare them against the element's subrole or they match
// nothing. The expected set comes from NSAccessibilityConstants.h
// (SearchField, Switch, ToolbarButton, TabButton — all Subrole constants,
// none declared as a role).
func TestAXSubroleNames_PinTheAppKitSubroleDeclarations(t *testing.T) {
	t.Parallel()

	want := []string{"AXSearchField", "AXSwitch", "AXTabButton", "AXToolbarButton"}

	got := make([]string, 0, len(element.AXSubroleNames))
	for name := range element.AXSubroleNames {
		got = append(got, name)
	}

	slices.Sort(got)

	if !slices.Equal(got, want) {
		t.Errorf("AXSubroleNames = %v, want %v", got, want)
	}
}

// TestAXSubroleNames_DoNotDoubleAsRoleNames guards the invariant the macOS
// matcher relies on: a configured name is compared against both the role and
// the subrole, which stays unambiguous only while no AX name appears in the
// vocabulary as a role in one mapping and a subrole in another.
func TestAXSubroleNames_DoNotDoubleAsRoleNames(t *testing.T) {
	t.Parallel()

	for _, mapping := range element.RoleVocabulary {
		if mapping.AXIsSubrole {
			continue
		}

		for _, name := range mapping.AX {
			if element.AXSubroleNames[name] {
				t.Errorf(
					"%q is a plain role name in mapping %q but also marked as a subrole",
					name, mapping.Semantic,
				)
			}
		}
	}
}
