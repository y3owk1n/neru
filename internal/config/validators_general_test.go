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
					ExecShell:     config.DefaultExecShell,
					ExecShellArgs: []string{config.DefaultExecShellFlag},
				},
			},
			wantErr: false,
		},
		{
			name: "kb layout id whitespace-only - invalid",
			config: config.Config{
				General: config.GeneralConfig{
					KBLayoutToUse: "   ",
					ExecShell:     config.DefaultExecShell,
					ExecShellArgs: []string{config.DefaultExecShellFlag},
				},
			},
			wantErr: true,
		},
		{
			name: "passthrough blacklist with modifier combo - valid",
			config: config.Config{
				General: config.GeneralConfig{
					PassthroughUnboundedKeysBlacklist: []string{"Cmd+W", "Ctrl+Space"},
					ExecShell:                         config.DefaultExecShell,
					ExecShellArgs:                     []string{config.DefaultExecShellFlag},
				},
			},
			wantErr: false,
		},
		{
			name: "passthrough blacklist without passthrough modifier - invalid",
			config: config.Config{
				General: config.GeneralConfig{
					PassthroughUnboundedKeysBlacklist: []string{"Shift+Tab"},
					ExecShell:                         config.DefaultExecShell,
					ExecShellArgs:                     []string{config.DefaultExecShellFlag},
				},
			},
			wantErr: true,
		},
		{
			name: "exec_shell is empty - invalid",
			config: config.Config{
				General: config.GeneralConfig{
					ExecShell:     "",
					ExecShellArgs: []string{config.DefaultExecShellFlag},
				},
			},
			wantErr: true,
		},
		{
			name: "exec_shell is relative path - invalid",
			config: config.Config{
				General: config.GeneralConfig{
					ExecShell:     "bash",
					ExecShellArgs: []string{config.DefaultExecShellFlag},
				},
			},
			wantErr: true,
		},
		{
			name: "exec_shell is absolute path - valid",
			config: config.Config{
				General: config.GeneralConfig{
					ExecShell:     "/usr/local/bin/fish",
					ExecShellArgs: []string{config.DefaultExecShellFlag},
				},
			},
			wantErr: false,
		},
		{
			name: "exec_shell_args is empty - invalid",
			config: config.Config{
				General: config.GeneralConfig{
					ExecShell:     config.DefaultExecShell,
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
