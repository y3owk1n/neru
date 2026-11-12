package loader_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
)

// TestLoad_ReadsVimKeymapsCaseSensitively pins [vim.keymaps] as written: H and
// h are different keys to vim, and the override file adds to the main file's
// mappings rather than replacing the table.
func TestLoad_ReadsVimKeymapsCaseSensitively(t *testing.T) {
	t.Parallel()

	result := loadWithOverride(t, `
[vim.keymaps.normal]
"H" = "^"
"h" = "<Nop>"

[vim.keymaps.visual]
"<CR>" = "y"
`, `
[vim.keymaps.normal]
"<Space>j" = "<Plug>(jump)"
`)
	normal := result.Config.Vim.Keymaps.Normal

	want := map[string]string{"H": "^", "h": "<Nop>", "<Space>j": "<Plug>(jump)"}
	for from, to := range want {
		if normal[from] != to {
			t.Errorf("vim.keymaps.normal[%q] = %q, want %q", from, normal[from], to)
		}
	}

	if got := result.Config.Vim.Keymaps.Visual["<CR>"]; got != "y" {
		t.Errorf("vim.keymaps.visual[<CR>] = %q, want y", got)
	}
}

// TestLoad_RefusesAVimMappingVimModeCannotMake covers the mappings a load
// refuses, each naming the table and the keys.
func TestLoad_RefusesAVimMappingVimModeCannotMake(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		toml string
		want string
	}{
		{
			name: "escape typed",
			toml: "[vim.keymaps.normal]\n\"<Esc>\" = \"<Nop>\"\n",
			want: `vim.keymaps.normal "<Esc>"`,
		},
		{
			name: "escape inside a sequence",
			toml: "[vim.keymaps.visual]\n\"g<Esc>\" = \"y\"\n",
			want: `vim.keymaps.visual "g<Esc>"`,
		},
		{
			name: "a key no tap names",
			toml: "[vim.keymaps.normal]\n\"<Del>\" = \"x\"\n",
			want: "not a key vim mode reads: <Del>",
		},
		{
			name: "nothing to run",
			toml: "[vim.keymaps.normal]\n\"Q\" = \"\"\n",
			want: "write <Nop>",
		},
		{
			name: "an unknown command",
			toml: "[vim.keymaps.normal]\n\"Q\" = \"<Plug>(jmup)\"\n",
			want: "not a command vim mode has: <Plug>(jmup)",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			configPath := filepath.Join(t.TempDir(), "config.toml")

			err := os.WriteFile(configPath, []byte(testCase.toml), 0o600)
			if err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			service := loader.NewService(config.DefaultConfig(), configPath, zap.NewNop(), nil)

			result := service.LoadWithValidation(configPath)
			if result.ValidationError == nil {
				t.Fatal("load accepted the mapping")
			}

			if !strings.Contains(result.ValidationError.Error(), testCase.want) {
				t.Errorf(
					"load error = %q, want it to say %q",
					result.ValidationError,
					testCase.want,
				)
			}
		})
	}
}
