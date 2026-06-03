package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestConfig_ValidateGeneral(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr bool
	}{
		{
			name: "kb layout id set - valid",
			config: config.Config{
				General: config.GeneralConfig{
					KBLayoutToUse: "com.apple.keylayout.ABC",
					ExecShell:     "/bin/bash",
					ExecShellArgs: []string{"-lc"},
				},
			},
			wantErr: false,
		},
		{
			name: "kb layout id whitespace-only - invalid",
			config: config.Config{
				General: config.GeneralConfig{
					KBLayoutToUse: "   ",
					ExecShell:     "/bin/bash",
					ExecShellArgs: []string{"-lc"},
				},
			},
			wantErr: true,
		},
		{
			name: "passthrough blacklist with modifier combo - valid",
			config: config.Config{
				General: config.GeneralConfig{
					PassthroughUnboundedKeysBlacklist: []string{"Cmd+W", "Ctrl+Space"},
					ExecShell:                         "/bin/bash",
					ExecShellArgs:                     []string{"-lc"},
				},
			},
			wantErr: false,
		},
		{
			name: "passthrough blacklist without passthrough modifier - invalid",
			config: config.Config{
				General: config.GeneralConfig{
					PassthroughUnboundedKeysBlacklist: []string{"Shift+Tab"},
					ExecShell:                         "/bin/bash",
					ExecShellArgs:                     []string{"-lc"},
				},
			},
			wantErr: true,
		},
		{
			name: "exec_shell is empty - invalid",
			config: config.Config{
				General: config.GeneralConfig{
					ExecShell:     "",
					ExecShellArgs: []string{"-lc"},
				},
			},
			wantErr: true,
		},
		{
			name: "exec_shell is relative path - invalid",
			config: config.Config{
				General: config.GeneralConfig{
					ExecShell:     "bash",
					ExecShellArgs: []string{"-lc"},
				},
			},
			wantErr: true,
		},
		{
			name: "exec_shell is absolute path - valid",
			config: config.Config{
				General: config.GeneralConfig{
					ExecShell:     "/usr/local/bin/fish",
					ExecShellArgs: []string{"-lc"},
				},
			},
			wantErr: false,
		},
		{
			name: "exec_shell_args is empty - invalid",
			config: config.Config{
				General: config.GeneralConfig{
					ExecShell:     "/bin/bash",
					ExecShellArgs: []string{},
				},
			},
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
