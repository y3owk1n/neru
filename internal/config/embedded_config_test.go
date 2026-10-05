package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/configs"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
)

// TestExampleConfigs_Validate loads each shipped config the way the daemon
// would. These files are copied by users verbatim, so a stale role vocabulary
// in one of them is shipped breakage that no other test sees — only
// default-config.toml is embedded and reachable through configs.DefaultConfig.
func TestExampleConfigs_Validate(t *testing.T) {
	for _, name := range configs.ShippedExamples {
		path := filepath.Join("..", "..", "configs", name)

		t.Run(name, func(t *testing.T) {
			svc := loader.NewService(config.DefaultConfig(), path, zap.NewNop(), nil)

			result := svc.LoadWithValidation(path)
			if result.ValidationError != nil {
				t.Fatalf("%s failed validation: %v", path, result.ValidationError)
			}

			// A config that enables hints must select something here, or the
			// user copies it and gets a blank overlay.
			if !result.Config.Hints.Enabled {
				return
			}

			if len(result.Config.Hints.ResolvedClickableRoles()) == 0 {
				t.Errorf(
					"%s enables hints but resolves to no clickable role on %s",
					path, runtime.GOOS,
				)
			}
		})
	}
}

// TestCrossPlatformConfig_LoadsButSelectsNothing pins the two halves of the
// cross-platform promise together. A config carrying only another platform's
// native roles must still load — that is what lets one dotfile serve several
// machines — but it must not then behave as though no filter was set.
func TestCrossPlatformConfig_LoadsButSelectsNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Every entry belongs to a platform that is not this one.
	foreign := map[string]string{
		goosDarwin: `["atspi:push button", "uia:Button"]`,
		"linux":    `["ax:AXButton", "uia:Button"]`,
		"windows":  `["ax:AXButton", "atspi:push button"]`,
	}[runtime.GOOS]

	if foreign == "" {
		t.Skipf("no foreign role set defined for %s", runtime.GOOS)
	}

	contents := `
[hints]
enabled = true
hint_characters = "asdf"
clickable_roles = ` + foreign + `
`

	err := os.WriteFile(path, []byte(contents), 0o600)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	svc := loader.NewService(config.DefaultConfig(), path, zap.NewNop(), nil)

	result := svc.LoadWithValidation(path)
	if result.ValidationError != nil {
		t.Fatalf(
			"a config of other platforms' roles must still load, got: %v",
			result.ValidationError,
		)
	}

	if roles := result.Config.Hints.ResolvedClickableRoles(); len(roles) != 0 {
		t.Errorf("ResolvedClickableRoles() = %v, want none to apply on %s", roles, runtime.GOOS)
	}
}
