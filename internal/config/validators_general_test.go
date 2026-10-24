package config_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

const goosWindows = "windows"

func validExecShell() (string, []string) {
	if runtime.GOOS == goosWindows {
		return filepath.Join("C:\\", "Windows", "System32", "cmd.exe"), []string{"/c"}
	}

	return config.DefaultExecShell, []string{config.DefaultExecShellFlag}
}

func TestConfig_ValidateGeneral(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr bool
	}{
		{
			name: "kb layout id set - valid",
			config: func() config.Config {
				shell, args := validExecShell()

				return config.Config{
					General: config.GeneralConfig{
						KBLayoutToUse:   "com.apple.keylayout.ABC",
						ExecShell:       shell,
						ExecShellArgs:   args,
						PrimaryModifier: config.DefaultConfig().General.PrimaryModifier,
					},
				}
			}(),
			wantErr: false,
		},
		{
			name: "kb layout id whitespace-only - invalid",
			config: func() config.Config {
				shell, args := validExecShell()

				return config.Config{
					General: config.GeneralConfig{
						KBLayoutToUse:   "   ",
						ExecShell:       shell,
						ExecShellArgs:   args,
						PrimaryModifier: config.DefaultConfig().General.PrimaryModifier,
					},
				}
			}(),
			wantErr: true,
		},
		{
			name: "passthrough blacklist with modifier combo - valid",
			config: func() config.Config {
				shell, args := validExecShell()

				return config.Config{
					General: config.GeneralConfig{
						PassthroughUnboundedKeysBlacklist: []string{"Cmd+W", "Ctrl+Space"},
						ExecShell:                         shell,
						ExecShellArgs:                     args,
						PrimaryModifier:                   config.DefaultConfig().General.PrimaryModifier,
					},
				}
			}(),
			wantErr: false,
		},
		{
			name: "passthrough blacklist without passthrough modifier - invalid",
			config: func() config.Config {
				shell, args := validExecShell()

				return config.Config{
					General: config.GeneralConfig{
						PassthroughUnboundedKeysBlacklist: []string{"Shift+Tab"},
						ExecShell:                         shell,
						ExecShellArgs:                     args,
						PrimaryModifier:                   config.DefaultConfig().General.PrimaryModifier,
					},
				}
			}(),
			wantErr: true,
		},
		{
			name: "exec_shell is empty - invalid",
			config: func() config.Config {
				_, args := validExecShell()

				return config.Config{
					General: config.GeneralConfig{
						ExecShell:       "",
						ExecShellArgs:   args,
						PrimaryModifier: config.DefaultConfig().General.PrimaryModifier,
					},
				}
			}(),
			wantErr: true,
		},
		{
			name: "exec_shell is relative path - invalid",
			config: func() config.Config {
				_, args := validExecShell()

				return config.Config{
					General: config.GeneralConfig{
						ExecShell:       "bash",
						ExecShellArgs:   args,
						PrimaryModifier: config.DefaultConfig().General.PrimaryModifier,
					},
				}
			}(),
			wantErr: true,
		},
		{
			name: "exec_shell is absolute path - valid",
			config: func() config.Config {
				shell, args := validExecShell()

				return config.Config{
					General: config.GeneralConfig{
						ExecShell:       shell,
						ExecShellArgs:   args,
						PrimaryModifier: config.DefaultConfig().General.PrimaryModifier,
					},
				}
			}(),
			wantErr: false,
		},
		{
			name: "exec_shell_args is empty - invalid",
			config: func() config.Config {
				shell, _ := validExecShell()

				return config.Config{
					General: config.GeneralConfig{
						ExecShell:       shell,
						ExecShellArgs:   []string{},
						PrimaryModifier: config.DefaultConfig().General.PrimaryModifier,
					},
				}
			}(),
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.config.ValidateGeneral()
			if (err != nil) != testCase.wantErr {
				t.Errorf("ValidateGeneral() error = %v, wantErr %v", err, testCase.wantErr)
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

			err := cfg.ValidateGeneral()
			if (err != nil) != testCase.wantErr {
				t.Errorf("ValidateGeneral() with linux = %q: error = %v, wantErr %v",
					testCase.linux, err, testCase.wantErr)
			}
		})
	}
}
