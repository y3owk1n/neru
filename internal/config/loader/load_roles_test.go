package loader_test

import (
	"runtime"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// TestLoadWithValidation_ReportsARoleThisPlatformCannotExpress is the report a
// user gets for the configuration this whole tier was built for: one written on
// a machine whose accessibility vocabulary has a role, carried to one whose
// vocabulary does not. The file loads, every other role keeps working, and the
// dead entry rides out on the result so `neru config validate` says so instead
// of "Configuration is valid" (#1376).
//
// The role has to be chosen per machine, and on macOS there is none to choose:
// every mapping in the vocabulary carries an AX name, so the diagnostic cannot
// arise there. What the pass reports on each platform is pinned without a
// machine to run it on in internal/config.
func TestLoadWithValidation_ReportsARoleThisPlatformCannotExpress(t *testing.T) {
	role, exists := roleWithNoNativeEquivalentHere()
	if !exists {
		t.Skipf("every semantic role resolves on %s", runtime.GOOS)
	}

	result, _ := loadWithObservedLogger(t, `
[hints]
clickable_roles = ["button", "`+role+`"]
`, "")

	if result.ValidationError != nil {
		t.Fatalf("a role that only fails to apply was refused: %v", result.ValidationError)
	}

	want := `hints.clickable_roles: "` + role + `" has no equivalent on ` +
		runtime.GOOS + " and is ignored"
	if !slices.Contains(result.Warnings, want) {
		t.Errorf("result.Warnings = %q, want it to contain %q", result.Warnings, want)
	}

	// The entry the user wrote survives: a warning reports, it does not undo.
	if !slices.Contains(result.Config.Hints.ClickableRoles, role) {
		t.Errorf("hints.clickable_roles = %q, want it left as written",
			result.Config.Hints.ClickableRoles)
	}
}

// TestLoadWithValidation_LeavesTheShippedRolesSilent is the gate that keeps the
// warning above from reaching everyone. The shipped role list is one list for
// every platform and several of its roles have no Linux or Windows equivalent,
// so a file that does not touch hints.clickable_roles must load without a word
// about them on any machine.
func TestLoadWithValidation_LeavesTheShippedRolesSilent(t *testing.T) {
	result, _ := loadWithObservedLogger(t, `
[hints.ui]
font_size = 21
`, "")

	if result.ValidationError != nil {
		t.Fatalf("a configuration on the shipped roles was refused: %v", result.ValidationError)
	}

	if len(result.Warnings) > 0 {
		t.Errorf("result.Warnings = %q, want none for the roles neru ships", result.Warnings)
	}
}

// roleWithNoNativeEquivalentHere returns the first semantic role that resolves
// to nothing on the running platform, and false when every one of them
// resolves.
func roleWithNoNativeEquivalentHere() (string, bool) {
	for _, mapping := range element.RoleVocabulary {
		entry := string(mapping.Semantic)
		if len(element.ResolveRolesForCurrentPlatform([]string{entry}).Native) == 0 {
			return entry, true
		}
	}

	return "", false
}
