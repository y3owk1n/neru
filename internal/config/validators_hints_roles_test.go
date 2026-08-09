package config_test

import (
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// TestValidateWithWarnings_ReportsARoleThisPlatformCannotExpress is the whole
// path a user sees: a role written in the file, a configuration that still
// loads, and a warning riding out on the sink `neru config validate` prints
// from. Which role that is depends on the machine, and on macOS there is none
// — every mapping carries an AX name — which is why the pass itself is pinned
// per platform in the package's own tests.
func TestValidateWithWarnings_ReportsARoleThisPlatformCannotExpress(t *testing.T) {
	t.Parallel()

	role, exists := roleWithNoNativeEquivalentHere()
	if !exists {
		t.Skipf("every semantic role resolves on %s", runtime.GOOS)
	}

	cfg := config.DefaultConfig()
	cfg.Hints.ClickableRoles = []string{TestRoleButton, role}

	warnings := &config.Warnings{}

	err := cfg.ValidateWithWarnings(warnings, config.WrittenConfig{})
	if err != nil {
		t.Fatalf("ValidateWithWarnings() refused a role that only fails to apply: %v", err)
	}

	want := []string{
		`hints.clickable_roles: "` + role + `" has no equivalent on ` + runtime.GOOS +
			" and is ignored",
	}
	if got := warnings.Messages(); !slices.Equal(got, want) {
		t.Errorf("warnings = %q, want %q", got, want)
	}
}

// TestValidateWithWarnings_ReportsAnApplicationRoleThisPlatformCannotExpress
// pins the same for the roles an individual application adds, which is the
// other half of the surface and the one a configuration copied between machines
// most often carries.
func TestValidateWithWarnings_ReportsAnApplicationRoleThisPlatformCannotExpress(t *testing.T) {
	t.Parallel()

	role, exists := roleWithNoNativeEquivalentHere()
	if !exists {
		t.Skipf("every semantic role resolves on %s", runtime.GOOS)
	}

	cfg := config.DefaultConfig()
	cfg.Hints.AppConfigs = []config.AppConfig{{
		BundleID:            bundleExample,
		AdditionalClickable: []string{role},
	}}

	warnings := &config.Warnings{}

	err := cfg.ValidateWithWarnings(warnings, config.WrittenConfig{})
	if err != nil {
		t.Fatalf("ValidateWithWarnings() refused a role that only fails to apply: %v", err)
	}

	want := []string{
		`hints.app_configs[0].additional_clickable_roles: "` + role +
			`" has no equivalent on ` + runtime.GOOS + " and is ignored",
	}
	if got := warnings.Messages(); !slices.Equal(got, want) {
		t.Errorf("warnings = %q, want %q", got, want)
	}
}

// TestValidateWithWarnings_LeavesRolesThisPlatformCanExpressSilent pins the
// other half of the report: a user who has rewritten both role lists, and whose
// every entry resolves, is told nothing. A warning tier that also fires on
// configurations with nothing wrong with them teaches people to ignore it.
func TestValidateWithWarnings_LeavesRolesThisPlatformCanExpressSilent(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.ClickableRoles = []string{TestRoleButton, TestRoleLink}
	cfg.Hints.AppConfigs = []config.AppConfig{{
		BundleID:            bundleExample,
		AdditionalClickable: []string{TestRoleTextField},
	}}

	warnings := &config.Warnings{}

	err := cfg.ValidateWithWarnings(warnings, config.WrittenConfig{})
	if err != nil {
		t.Fatalf("ValidateWithWarnings() refused a resolvable configuration: %v", err)
	}

	if got := warnings.Messages(); len(got) > 0 {
		t.Errorf("warnings = %q, want none", got)
	}
}

// TestValidateWithWarnings_KeepsRefusingAnUnknownRole pins the tier line: an
// entry naming no role at all is still the whole file's problem, and a refusal
// says nothing on the side. Warnings describe the configuration that loaded,
// and this one did not.
func TestValidateWithWarnings_KeepsRefusingAnUnknownRole(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.ClickableRoles = []string{TestRoleButton, "AXButton"}

	warnings := &config.Warnings{}

	err := cfg.ValidateWithWarnings(warnings, config.WrittenConfig{})
	if err == nil {
		t.Fatal("ValidateWithWarnings() accepted a role name nothing recognizes")
	}

	if got := err.Error(); !strings.Contains(got, `unknown role "AXButton"`) {
		t.Errorf("error = %q, want it to name the unknown role", got)
	}

	if got := warnings.Messages(); len(got) > 0 {
		t.Errorf("warnings = %q, want none alongside a refusal", got)
	}
}

// TestValidateWithWarnings_NamesTheApplicationWhoseRoleIsUnknown pins the
// refusal's half of the same question the warning above answers: which of a
// file's overrides to go and edit. Two of them are configured and the second is
// the one at fault, so a message that named the field without the index — or
// named the first one — fails here.
func TestValidateWithWarnings_NamesTheApplicationWhoseRoleIsUnknown(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.AppConfigs = []config.AppConfig{
		{
			BundleID:            bundleExample,
			AdditionalClickable: []string{TestRoleTextField},
		},
		{
			BundleID:            bundleOther,
			AdditionalClickable: []string{"AXButton"},
		},
	}

	err := cfg.ValidateWithWarnings(&config.Warnings{}, config.WrittenConfig{})
	if err == nil {
		t.Fatal("ValidateWithWarnings() accepted a role name nothing recognizes")
	}

	want := `hints.app_configs[1].additional_clickable_roles: unknown role "AXButton"`
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("error = %q, want it to contain %q", got, want)
	}
}

// roleWithNoNativeEquivalentHere returns the first semantic role that resolves
// to nothing on the running platform, and false when every one of them
// resolves. It reads the vocabulary rather than naming a role, so a mapping
// that gains a native name stops being this test's example instead of failing
// it for the wrong reason.
func roleWithNoNativeEquivalentHere() (string, bool) {
	for _, mapping := range element.RoleVocabulary {
		entry := string(mapping.Semantic)
		if len(element.ResolveRolesForCurrentPlatform([]string{entry}).Native) == 0 {
			return entry, true
		}
	}

	return "", false
}
