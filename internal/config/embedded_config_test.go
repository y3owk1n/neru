package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/configs"
	"github.com/y3owk1n/neru/internal/config"
)

// TestEmbeddedDefaultConfig_Validates guards the shipped configuration against
// the role vocabulary. A default config written in a stale role vocabulary
// loads to zero hints, which is invisible until a user reports blank overlays.
func TestEmbeddedDefaultConfig_Validates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	err := os.WriteFile(path, configs.DefaultConfig, 0o600)
	if err != nil {
		t.Fatalf("failed to write embedded config: %v", err)
	}

	service := config.NewService(config.DefaultConfig(), path, zap.NewNop(), nil)

	result := service.LoadWithValidation(path)
	if result.ValidationError != nil {
		t.Fatalf("embedded default config failed validation: %v", result.ValidationError)
	}

	roles := result.Config.Hints.ResolvedClickableRoles()
	if len(roles) == 0 {
		t.Errorf(
			"embedded default config resolves to no clickable roles on %s; "+
				"hints would be blank",
			result.Config.Hints.ClickableRoles,
		)
	}
}

// shippedExampleConfigs are the configs the project publishes under configs/.
//
// They are listed rather than globbed because the directory is also a working
// area: a local config kept there for testing is not a project artifact, and
// its problems are not this suite's business. Adding an example here is the
// deliberate step that puts it under test.
var shippedExampleConfigs = []string{
	"default-config.toml",
	"grid-only-config.toml",
	"hints-only-config.toml",
	"recursive-grid-only-config.toml",
}

// TestExampleConfigs_Validate loads each shipped config the way the daemon
// would. These files are copied by users verbatim, so a stale role vocabulary
// in one of them is shipped breakage that no other test sees — only
// default-config.toml is embedded and reachable through configs.DefaultConfig.
func TestExampleConfigs_Validate(t *testing.T) {
	for _, name := range shippedExampleConfigs {
		path := filepath.Join("..", "..", "configs", name)

		t.Run(name, func(t *testing.T) {
			svc := config.NewService(config.DefaultConfig(), path, zap.NewNop(), nil)

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

	svc := config.NewService(config.DefaultConfig(), path, zap.NewNop(), nil)

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
