package config_test

import (
	"runtime"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

const goosWindows = "windows"

// hostShell is an absolute path to a shell on the machine running the test,
// which is what the deprecated general.exec_shell is checked against.
func hostShell() string {
	if runtime.GOOS == goosWindows {
		return TestWindowsCmd
	}

	return TestShellZsh
}

// runningShellEntry is the general.shell entry the machine running the test
// reads.
func runningShellEntry(general *config.GeneralConfig) *[]string {
	switch runtime.GOOS {
	case "darwin":
		return &general.Shell.Darwin
	case goosWindows:
		return &general.Shell.Windows
	default:
		return &general.Shell.Linux
	}
}

func TestConfig_ValidateGeneral(t *testing.T) {
	tests := []struct {
		name        string
		change      func(general *config.GeneralConfig)
		wantErr     bool
		wantWarning string
	}{
		{
			name:   "defaults - valid",
			change: func(*config.GeneralConfig) {},
		},
		{
			name: "kb layout id set - valid",
			change: func(general *config.GeneralConfig) {
				general.KBLayoutToUse.Darwin = "com.apple.keylayout.ABC"
			},
		},
		{
			name: "kb layout id whitespace-only - invalid",
			change: func(general *config.GeneralConfig) {
				general.KBLayoutToUse.Linux = "   "
			},
			wantErr: true,
		},
		{
			name: "passthrough blacklist with modifier combo - valid",
			change: func(general *config.GeneralConfig) {
				general.PassthroughUnboundedKeysBlacklist = []string{"Cmd+W", "Ctrl+Space"}
			},
		},
		{
			name: "passthrough blacklist without passthrough modifier - invalid",
			change: func(general *config.GeneralConfig) {
				general.PassthroughUnboundedKeysBlacklist = []string{"Shift+Tab"}
			},
			wantErr: true,
		},
		{
			name: "this platform's shell is a relative path - invalid",
			change: func(general *config.GeneralConfig) {
				*runningShellEntry(general) = []string{"bash", TestShellFlag}
			},
			wantErr: true,
		},
		{
			name: "one shell value for every platform - valid on this one",
			change: func(general *config.GeneralConfig) {
				argv := []string{hostShell(), TestShellFlag}
				general.Shell = config.PerPlatform[[]string]{
					Darwin:  argv,
					Linux:   argv,
					Windows: argv,
				}
			},
		},
		{
			name: "shell entry has no argument - invalid",
			change: func(general *config.GeneralConfig) {
				general.Shell.Darwin = []string{TestShellZsh}
			},
			wantErr: true,
		},
		{
			name: "deprecated exec_shell absolute - valid, warned",
			change: func(general *config.GeneralConfig) {
				shell := hostShell()
				general.ExecShell = &shell
			},
			wantWarning: "general.exec_shell is deprecated and is removed in v2. Move it to general.shell",
		},
		{
			name: "deprecated exec_shell relative - invalid",
			change: func(general *config.GeneralConfig) {
				shell := "bash"
				general.ExecShell = &shell
			},
			wantErr: true,
		},
		{
			name: "deprecated exec_shell_args set - valid, warned",
			change: func(general *config.GeneralConfig) {
				general.ExecShellArgs = &[]string{"-c"}
			},
			wantWarning: "general.exec_shell_args is deprecated and is removed in v2. Move it to general.shell",
		},
		{
			name: "deprecated exec_shell_args empty - unset",
			change: func(general *config.GeneralConfig) {
				general.ExecShellArgs = &[]string{}
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			testCase.change(&cfg.General)

			var warnings config.Warnings

			err := cfg.ValidateGeneral(&warnings)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("ValidateGeneral() error = %v, wantErr %v", err, testCase.wantErr)
			}

			if testCase.wantWarning != "" &&
				!slices.Contains(warnings.Messages(), testCase.wantWarning) {
				t.Errorf("warnings = %q, want %q", warnings.Messages(), testCase.wantWarning)
			}
		})
	}
}

// TestGeneralConfig_ShellCommand_DeprecatedKeysReplaceTheirPart pins what the
// deprecated keys mean until v2: each one, when set, replaces its part of the
// shell command on every platform.
func TestGeneralConfig_ShellCommand_DeprecatedKeysReplaceTheirPart(t *testing.T) {
	tests := []struct {
		name  string
		shell string
		args  []string
		want  []string
	}{
		{name: "neither", want: []string{TestShellBash, TestShellFlag}},
		{name: "shell only", shell: TestShellZsh, want: []string{TestShellZsh, TestShellFlag}},
		{name: "args only", args: []string{"-c"}, want: []string{TestShellBash, "-c"}},
		{
			name:  "both",
			shell: TestShellZsh,
			args:  []string{"-c"},
			want:  []string{TestShellZsh, "-c"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			general := config.DefaultConfig().General
			if testCase.shell != "" {
				general.ExecShell = &testCase.shell
			}

			if testCase.args != nil {
				general.ExecShellArgs = &testCase.args
			}

			if got := general.ShellCommand(TestGOOSLinux); !slices.Equal(got, testCase.want) {
				t.Errorf("ShellCommand(linux) = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestConfig_ValidateGeneral_PrimaryModifier pins what Primary may stand for.
// Every entry is checked on every platform, so a shared file that is wrong for
// another machine is refused on this one too.
func TestConfig_ValidateGeneral_PrimaryModifier(t *testing.T) {
	tests := []struct {
		name    string
		linux   string
		wantErr bool
	}{
		{name: "shipped default", linux: "Ctrl", wantErr: false},
		{name: "alias in any case", linux: "super", wantErr: false},
		{name: "alt alias", linux: "Option", wantErr: false},
		{name: "primary is circular", linux: "Primary", wantErr: true},
		{name: "shift types capitals", linux: "Shift", wantErr: true},
		{name: "sided modifier", linux: "LeftCtrl", wantErr: true},
		{name: "unknown modifier", linux: "Hyper", wantErr: true},
		{name: "empty", linux: "", wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.General.PrimaryModifier.Linux = testCase.linux

			err := cfg.ValidateGeneral(nil)
			if (err != nil) != testCase.wantErr {
				t.Errorf("ValidateGeneral() with linux = %q: error = %v, wantErr %v",
					testCase.linux, err, testCase.wantErr)
			}
		})
	}
}
