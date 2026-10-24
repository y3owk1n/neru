package loader_test

import (
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
)

// TestLoadWithValidation_PerPlatformTableKeepsUnwrittenEntries pins that a
// table naming one platform leaves the other two at their defaults.
func TestLoadWithValidation_PerPlatformTableKeepsUnwrittenEntries(t *testing.T) {
	result, _ := loadWithObservedLogger(t, `
[general.shell]
linux = ["/bin/zsh", "-c"]
`, "")

	if result.ValidationError != nil {
		t.Fatalf("load refused: %v", result.ValidationError)
	}

	shell := result.Config.General.Shell
	if !slices.Equal(shell.Linux, []string{"/bin/zsh", "-c"}) ||
		!slices.Equal(shell.Darwin, []string{"/bin/bash", "-lc"}) ||
		!slices.Equal(shell.Windows, []string{`C:\Windows\System32\cmd.exe`, "/c"}) {
		t.Errorf(
			"general.shell = %+v, want linux set and darwin and windows at their defaults",
			shell,
		)
	}
}

// TestLoadWithValidation_PerPlatformSingleValueAppliesEverywhere pins the
// single-value form, which is also what an existing kb_layout_to_use line
// written before the option became per-platform loads as.
func TestLoadWithValidation_PerPlatformSingleValueAppliesEverywhere(t *testing.T) {
	result, _ := loadWithObservedLogger(t, `
[general]
kb_layout_to_use = "com.apple.keylayout.Dvorak"
`, "")

	if result.ValidationError != nil {
		t.Fatalf("load refused: %v", result.ValidationError)
	}

	layout := result.Config.General.KBLayoutToUse
	for _, got := range []string{layout.Darwin, layout.Linux, layout.Windows} {
		if got != "com.apple.keylayout.Dvorak" {
			t.Errorf("kb_layout_to_use = %+v, want the one value on every platform", layout)

			break
		}
	}
}

// TestLoadWithValidation_PerPlatformRefusesAKeyThatIsNoPlatform pins that a
// misspelled platform is refused rather than dropped, since the decoder hands
// the table over whole and cannot report the key as unknown.
func TestLoadWithValidation_PerPlatformRefusesAKeyThatIsNoPlatform(t *testing.T) {
	result, _ := loadWithObservedLogger(t, `
[general.kb_layout_to_use]
macos = "com.apple.keylayout.Dvorak"
`, "")

	if result.ValidationError == nil ||
		!strings.Contains(result.ValidationError.Error(), `"macos"`) {
		t.Fatalf("ValidationError = %v, want it to name the key macos", result.ValidationError)
	}
}

// TestLoadWithValidation_OneShellValueLoadsOnThisPlatform pins that one shell
// for every platform loads, though it cannot be a valid path on all three.
// Only the entry this machine reads has to be.
func TestLoadWithValidation_OneShellValueLoadsOnThisPlatform(t *testing.T) {
	shell := hostAbsoluteShell()

	result, _ := loadWithObservedLogger(t, `
[general]
shell = ['`+shell+`', "-c"]
`, "")

	if result.ValidationError != nil {
		t.Fatalf("load refused: %v", result.ValidationError)
	}

	want := []string{shell, "-c"}
	if got := result.Config.General.Shell.Windows; !slices.Equal(got, want) {
		t.Errorf("shell.windows = %q, want the one value %q", got, want)
	}
}

// TestLoadWithValidation_DeprecatedShellKeysStillApply pins that a config
// written before general.shell keeps running its shell until v2, and is told
// to move the line.
func TestLoadWithValidation_DeprecatedShellKeysStillApply(t *testing.T) {
	shell := hostAbsoluteShell()

	result, _ := loadWithObservedLogger(t, `
[general]
exec_shell = '`+shell+`'
exec_shell_args = ["-c"]
`, "")

	if result.ValidationError != nil {
		t.Fatalf("load refused: %v", result.ValidationError)
	}

	want := []string{shell, "-c"}
	if got := result.Config.General.ShellCommand("darwin"); !slices.Equal(got, want) {
		t.Errorf("ShellCommand(darwin) = %q, want the deprecated keys' %q", got, want)
	}

	for _, key := range []string{"general.exec_shell", "general.exec_shell_args"} {
		want := key + " is deprecated and is removed in v2. Move it to general.shell"
		if !slices.Contains(result.Warnings, want) {
			t.Errorf("result.Warnings = %q, want %q", result.Warnings, want)
		}
	}
}

// TestSetField_ReachesPerPlatformEntriesAndDeprecatedShellKeys pins the paths
// `neru config set` takes for these options: one platform's entry, the whole
// option, and the deprecated keys until v2.
func TestSetField_ReachesPerPlatformEntriesAndDeprecatedShellKeys(t *testing.T) {
	const layout = "00010409"

	cfg := config.DefaultConfig()

	for path, value := range map[string]string{
		"general.shell.linux":      `["/bin/zsh", "-c"]`,
		"general.kb_layout_to_use": layout,
		"general.exec_shell":       "/bin/fish",
	} {
		err := loader.SetField(cfg, path, value)
		if err != nil {
			t.Fatalf("SetField(%s): %v", path, err)
		}
	}

	if got := cfg.General.ShellCommand("linux"); !slices.Equal(got, []string{"/bin/fish", "-c"}) {
		t.Errorf("ShellCommand(linux) = %q, want the deprecated shell over the entry's -c", got)
	}

	// The whole option takes one value for every platform, as the file's
	// single-value form does, so a `config set general.kb_layout_to_use` written
	// before the option became per-platform still works.
	want := config.PerPlatform[string]{Darwin: layout, Linux: layout, Windows: layout}
	if got := cfg.General.KBLayoutToUse; got != want {
		t.Errorf("kb_layout_to_use = %+v, want %+v", got, want)
	}
}

// hostAbsoluteShell is an absolute path on the machine running the test, which
// is what the running platform's shell entry and the deprecated exec_shell are
// checked against.
func hostAbsoluteShell() string {
	if runtime.GOOS == "windows" {
		return `C:\Windows\System32\cmd.exe`
	}

	return "/bin/zsh"
}
