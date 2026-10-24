package loader_test

import (
	"strings"
	"testing"
)

// TestLoadWithValidation_PrimaryModifierKeepsUnwrittenEntries pins that a
// table naming one platform leaves the other two at their defaults, so a user
// changing Linux does not silently unbind macOS.
func TestLoadWithValidation_PrimaryModifierKeepsUnwrittenEntries(t *testing.T) {
	result, _ := loadWithObservedLogger(t, `
[general]
primary_modifier = { linux = "Super" }
`, "")

	if result.ValidationError != nil {
		t.Fatalf("load refused: %v", result.ValidationError)
	}

	got := result.Config.General.PrimaryModifier
	if got.Linux != "Super" || got.Darwin != "Cmd" || got.Windows != "Ctrl" {
		t.Errorf(
			"primary_modifier = %+v, want linux Super with darwin Cmd and windows Ctrl kept",
			got,
		)
	}
}

// TestLoadWithValidation_PrimaryModifierComparesUnderTheFilesOwnTable pins
// that a file's hotkeys are compared under the table it declares, not the one
// the running daemon was started with. Primary as Alt makes these two the same
// chord, and only one of them could ever fire.
func TestLoadWithValidation_PrimaryModifierComparesUnderTheFilesOwnTable(t *testing.T) {
	tests := []struct {
		name    string
		general string
		wantErr bool
	}{
		{name: "default table keeps them apart", general: "", wantErr: false},
		{
			name:    "alt table makes them one chord",
			general: `primary_modifier = { darwin = "Alt", linux = "Alt", windows = "Alt" }`,
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result, _ := loadWithObservedLogger(t, `
[general]
`+testCase.general+`

[hotkeys]
"Primary+Shift+X" = "hints"
"Alt+Shift+X" = "grid"
`, "")

			err := result.ValidationError
			if (err != nil) != testCase.wantErr {
				t.Fatalf("ValidationError = %v, wantErr %v", err, testCase.wantErr)
			}

			if err != nil && !strings.Contains(err.Error(), "duplicate") {
				t.Errorf("ValidationError = %v, want it to name the duplicate", err)
			}
		})
	}
}

// TestLoadWithValidation_PrimaryModifierFromTheOverrideAppliesToTheFile pins
// that a table `neru config set` saved is in force when the main file's
// hotkeys are compared. Primary as Alt keeps these two apart. Compared under
// the file's own Ctrl they collide, and the load would be refused.
func TestLoadWithValidation_PrimaryModifierFromTheOverrideAppliesToTheFile(t *testing.T) {
	result, _ := loadWithObservedLogger(t, `
[general]
primary_modifier = { darwin = "Ctrl", linux = "Ctrl", windows = "Ctrl" }

[hotkeys]
"Primary+Shift+X" = "hints"
"Ctrl+Shift+X" = "grid"
`, `
[general.primary_modifier]
darwin = "Alt"
linux = "Alt"
windows = "Alt"
`)

	if result.ValidationError != nil {
		t.Fatalf("load refused: %v", result.ValidationError)
	}

	if got := result.Config.General.PrimaryModifier.Linux; got != "Alt" {
		t.Errorf("primary_modifier.linux = %q, want the override's Alt", got)
	}
}
