package config_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

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
