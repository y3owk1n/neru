package config

import (
	"slices"
	"testing"
)

// The platform a role list is judged against is an argument to the pass rather
// than something a test can arrange, so these run against the unexported form
// that takes one. The alternative is a suite that can only assert this
// behavior on the two platforms where it happens — a semantic role with no
// native equivalent cannot exist on macOS, where every mapping carries an AX
// name — which would leave the primary platform's CI blind to it.
const (
	goosDarwin  = "darwin"
	goosLinux   = "linux"
	goosWindows = "windows"
)

// A role every platform can express, as the company an unresolvable one keeps:
// a list is only reported entry by entry if the rest of it survives.
const testRoleButton = "button"

// TestConfig_WarnUnresolvableClickableRoles_NamesEntriesThatResolveToNothing
// pins what a user is told about a role that this platform's accessibility
// vocabulary has no counterpart for: the entry is named, and so is the field it
// was written in.
func TestConfig_WarnUnresolvableClickableRoles_NamesEntriesThatResolveToNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		goos  string
		roles []string
		want  []string
	}{
		{
			// AT-SPI has no disclosure triangle; the other two resolve.
			name:  "a role AT-SPI cannot express",
			goos:  goosLinux,
			roles: []string{testRoleButton, "disclosure", "link"},
			want: []string{
				`hints.clickable_roles: "disclosure" has no equivalent on linux and is ignored`,
			},
		},
		{
			// UI Automation has neither, and each is reported on its own so a
			// list with two mistakes does not hide one behind the other.
			name:  "two roles UI Automation cannot express",
			goos:  goosWindows,
			roles: []string{"menu_button", testRoleButton, "switch"},
			want: []string{
				`hints.clickable_roles: "menu_button" has no equivalent on windows and is ignored`,
				`hints.clickable_roles: "switch" has no equivalent on windows and is ignored`,
			},
		},
		{
			name:  "roles this platform can express",
			goos:  goosLinux,
			roles: []string{testRoleButton, "link", "text_field"},
			want:  nil,
		},
		{
			// A native entry addressed to another platform is how one
			// configuration serves several machines, and stays silent on
			// purpose: it is not a mistake, it is the other machine's line.
			name:  "a native entry belonging to another platform",
			goos:  goosDarwin,
			roles: []string{testRoleButton, "atspi:push button", "uia:Button"},
			want:  nil,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := DefaultConfig()
			cfg.Hints.ClickableRoles = testCase.roles

			warnings := &Warnings{}
			cfg.warnUnresolvableClickableRoles(warnings, testCase.goos)

			if got := warnings.Messages(); !slices.Equal(got, testCase.want) {
				t.Errorf("warnings = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestConfig_WarnUnresolvableClickableRoles_LeavesTheShippedListAlone is the
// gate the whole pass hangs on. The roles neru ships are one list for every
// platform, and some of them resolve to nothing on Linux and Windows — warning
// about those would greet every user of two platforms with a complaint about a
// file they have not written a line of.
func TestConfig_WarnUnresolvableClickableRoles_LeavesTheShippedListAlone(t *testing.T) {
	t.Parallel()

	for _, goos := range []string{goosDarwin, goosLinux, goosWindows} {
		t.Run(goos, func(t *testing.T) {
			t.Parallel()

			warnings := &Warnings{}
			DefaultConfig().warnUnresolvableClickableRoles(warnings, goos)

			if got := warnings.Messages(); len(got) > 0 {
				t.Errorf("warnings = %q, want none for the shipped roles", got)
			}
		})
	}
}

// TestConfig_WarnUnresolvableClickableRoles_ReadsAnApplicationsExtraRoles pins
// the other place a role list is written. An application's extra roles are
// never shipped, so every entry in one is the user's own and there is nothing
// to compare against.
func TestConfig_WarnUnresolvableClickableRoles_ReadsAnApplicationsExtraRoles(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Hints.AppConfigs = []AppConfig{{
		BundleID:            "com.apple.Safari",
		AdditionalClickable: []string{testRoleButton, "toolbar_button"},
	}}

	warnings := &Warnings{}
	cfg.warnUnresolvableClickableRoles(warnings, goosLinux)

	want := []string{
		`hints.app_configs.additional_clickable_roles: ` +
			`"toolbar_button" has no equivalent on linux and is ignored`,
	}
	if got := warnings.Messages(); !slices.Equal(got, want) {
		t.Errorf("warnings = %q, want %q", got, want)
	}
}
