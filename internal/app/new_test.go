package app_test

import (
	"slices"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports/mocks"
)

// TestNew_ServicesPhaseFailureUnwindsWithoutPanicking checks that New returns
// the error from a failed services phase instead of panicking in the unwind.
// The infrastructure cleanup runs before phase 7 has built the hotkey binder.
// Each case is a hint alphabet the services phase rejects.
//
// The cases run serially because New writes package-level state, such as the
// primary modifier.
func TestNew_ServicesPhaseFailureUnwindsWithoutPanicking(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(cfg *config.Config)
		wantCode derrors.Code
	}{
		{
			name:     "hint alphabet shorter than two characters",
			mutate:   func(cfg *config.Config) { cfg.Hints.HintCharacters = "a" },
			wantCode: derrors.CodeHintGenerationFailed,
		},
		{
			// Two characters pass the length check, but the generator folds case
			// and keeps one.
			name:     "hint alphabet with one character after case folding",
			mutate:   func(cfg *config.Config) { cfg.Hints.HintCharacters = "aA" },
			wantCode: derrors.CodeHintGenerationFailed,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			testCase.mutate(cfg)

			application, err := app.New(
				app.WithConfig(cfg),
				app.WithLogger(zap.NewNop()),
				app.WithEventTap(&mocks.MockEventTapPort{}),
				app.WithTextInput(&mocks.MockTextInputPort{}),
				app.WithIPCServer(&mocks.MockIPCPort{}),
				app.WithWatcher(&mocks.MockAppWatcherPort{}),
				app.WithHotkeyService(&mocks.MockHotkeyPort{}),
				app.WithOverlayPort(&mocks.MockOverlayPort{}),
				app.WithSystemPort(&mocks.MockSystemPort{}),
				app.WithAccessibility(&mocks.MockAccessibilityPort{}),
				app.WithMouse(&mocks.MockMousePort{}),
				app.WithVisionPort(&mocks.MockVisionPort{}),
			)

			if application != nil {
				t.Error("New returned an App for a startup that failed")
			}

			if !derrors.IsCode(err, testCase.wantCode) {
				t.Fatalf("New error = %v, want code %s", err, testCase.wantCode)
			}
		})
	}
}

// TestNew_HandsTheConfiguredClickableRolesToTheAccessibilityPort checks that
// startup applies hints.clickable_roles through the accessibility port, so
// the roles a user configures take effect before the first reload.
func TestNew_HandsTheConfiguredClickableRolesToTheAccessibilityPort(t *testing.T) {
	tests := []struct {
		name         string
		hintsEnabled bool
	}{
		{name: "hints enabled", hintsEnabled: true},
		{name: "hints disabled", hintsEnabled: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Hints.Enabled = testCase.hintsEnabled
			cfg.Hints.ClickableRoles = []string{"button", "ax:AXCell"}

			var applied [][]string

			axPort := &mocks.MockAccessibilityPort{
				UpdateClickableRolesFunc: func(roles []string) {
					applied = append(applied, roles)
				},
			}

			application, err := app.New(
				app.WithConfig(cfg),
				app.WithConfigPath(""),
				app.WithLogger(zap.NewNop()),
				app.WithEventTap(&mocks.MockEventTapPort{}),
				app.WithTextInput(&mocks.MockTextInputPort{}),
				app.WithIPCServer(&mocks.MockIPCPort{}),
				app.WithWatcher(&mocks.MockAppWatcherPort{}),
				app.WithHotkeyService(&mocks.MockHotkeyPort{}),
				app.WithOverlayPort(&mocks.MockOverlayPort{}),
				app.WithSystemPort(&mocks.MockSystemPort{}),
				app.WithAccessibility(axPort),
				app.WithMouse(&mocks.MockMousePort{}),
				app.WithVisionPort(&mocks.MockVisionPort{}),
				app.WithClipboard(&mocks.MockClipboardPort{}),
				app.WithKeyFeed(&mocks.MockKeyFeedPort{}),
			)
			if err != nil {
				t.Fatalf("New failed: %v", err)
			}

			t.Cleanup(application.Cleanup)

			var want [][]string
			if testCase.hintsEnabled {
				want = [][]string{cfg.Hints.ResolvedClickableRoles()}
			}

			if !slices.EqualFunc(applied, want, slices.Equal[[]string]) {
				t.Fatalf("startup applied clickable roles %v, want %v", applied, want)
			}
		})
	}
}
