package app_test

import (
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
// A one-character hint alphabet fails the services phase.
func TestNew_ServicesPhaseFailureUnwindsWithoutPanicking(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.HintCharacters = "a"

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
		app.WithVisionPort(&mocks.MockVisionPort{}),
	)

	if application != nil {
		t.Error("New returned an App for a startup that failed")
	}

	if !derrors.IsCode(err, derrors.CodeHintGenerationFailed) {
		t.Fatalf("New error = %v, want code %s", err, derrors.CodeHintGenerationFailed)
	}
}
