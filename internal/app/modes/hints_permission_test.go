package modes

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/y3owk1n/neru/internal/app/components"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/app/services"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// permissionModal is a system mock whose screen-capture permission is missing
// and whose blocking permission dialog stays open until the test answers it.
type permissionModal struct {
	system  *portmocks.MockSystemPort
	started chan struct{}
	consent chan ports.ScreenCaptureConsent
}

func newPermissionModal() *permissionModal {
	modal := &permissionModal{
		started: make(chan struct{}),
		consent: make(chan ports.ScreenCaptureConsent),
	}

	modal.system = &portmocks.MockSystemPort{
		CheckScreenCapturePermissionFunc: func(context.Context) bool { return false },
		RequestScreenCapturePermissionFunc: func(context.Context) ports.ScreenCaptureConsent {
			close(modal.started)

			return <-modal.consent
		},
	}

	return modal
}

// awaitOpen fails the test unless the permission dialog gets requested.
func (m *permissionModal) awaitOpen(t *testing.T) {
	t.Helper()

	select {
	case <-m.started:
	case <-time.After(2 * time.Second):
		t.Fatal("permission dialog was never requested")
	}
}

// newVisionHintsHandler builds a handler whose hint activation reaches the
// screen-capture permission gate: vision strategy with permission not granted.
func newVisionHintsHandler(
	systemMock *portmocks.MockSystemPort,
	logger *zap.Logger,
	shutdown func(),
) *Handler {
	return newCaptureHintsHandler(domain.StrategyVision, systemMock, logger, shutdown)
}

// newCaptureHintsHandler builds a handler whose configured hint strategy is
// strategy and whose screen-capture permission comes from systemMock.
func newCaptureHintsHandler(
	strategy string,
	systemMock *portmocks.MockSystemPort,
	logger *zap.Logger,
	shutdown func(),
) *Handler {
	handler := newHandlerWithState(handlerState{
		ctx:    context.Background(),
		logger: logger,
		config: &configpkg.Config{
			Hints: configpkg.HintsConfig{
				Enabled:  true,
				Strategy: strategy,
			},
		},
		appState:      state.NewAppState(),
		cursorState:   state.NewCursorState(),
		modifierState: state.NewModifierState(),
		system:        systemMock,
		actionService: services.NewActionService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockOverlayPort{},
			systemMock,
			zap.NewNop(),
		),
		hints:    &components.HintsComponent{Context: &hintscomponent.Context{}},
		scroll:   &components.ScrollComponent{Context: &scroll.Context{}},
		shutdown: shutdown,
	})

	handler.modes = map[domain.Mode]Mode{
		domain.ModeHints: NewHintsMode(&handler.handlerState),
	}

	return handler
}

func TestActivateMode_PermissionDialogDoesNotBlockOrHoldLock(t *testing.T) {
	modal := newPermissionModal()
	shutdownCalled := make(chan struct{})

	handler := newVisionHintsHandler(modal.system, zap.NewNop(), func() { close(shutdownCalled) })

	// Must return with the dialog still open: the blocking permission request
	// runs on its own goroutine, never on the activation path under h.mu.
	handler.ActivateMode(modecmd.Activation{Mode: domain.ModeHints})

	modal.awaitOpen(t)

	if !handler.mu.TryLock() {
		t.Fatal("handler lock held while the permission dialog is open")
	}
	handler.mu.Unlock()

	modal.consent <- ports.ScreenCaptureQuit

	select {
	case <-shutdownCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("quit consent never reached shutdown")
	}
}

func TestActivateMode_ContourStrategyRequestsScreenCapturePermission(t *testing.T) {
	modal := newPermissionModal()
	handler := newCaptureHintsHandler(domain.StrategyContour, modal.system, zap.NewNop(), func() {})

	// Contour reads the window's pixels just like vision does, so an
	// activation without consent must raise the dialog instead of letting the
	// capture fail under the mode handler's lock.
	handler.ActivateMode(modecmd.Activation{Mode: domain.ModeHints})

	modal.awaitOpen(t)

	modal.consent <- ports.ScreenCaptureCanceled
}

func TestResumeHintActivationAfterPermission_DoesNotRepromptWhenCheckStillFails(t *testing.T) {
	requests := make(chan struct{}, 2)
	consent := make(chan ports.ScreenCaptureConsent)

	systemMock := &portmocks.MockSystemPort{
		// The check keeps failing even after a Granted consent, violating the
		// SystemPort contract; the resume must drop the consent rather than
		// re-run the activation and show the dialog again forever.
		CheckScreenCapturePermissionFunc: func(context.Context) bool { return false },
		RequestScreenCapturePermissionFunc: func(context.Context) ports.ScreenCaptureConsent {
			requests <- struct{}{}

			return <-consent
		},
	}

	core, logs := observer.New(zap.DebugLevel)

	handler := newVisionHintsHandler(systemMock, zap.New(core), nil)

	handler.ActivateMode(modecmd.Activation{Mode: domain.ModeHints})

	select {
	case <-requests:
	case <-time.After(2 * time.Second):
		t.Fatal("permission dialog was never requested")
	}

	consent <- ports.ScreenCaptureGranted

	deadline := time.Now().Add(2 * time.Second)
	for logs.FilterMessageSnippet("permission check still fails").Len() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("contract-violating consent was never dropped")
		}

		time.Sleep(2 * time.Millisecond)
	}

	select {
	case <-requests:
		t.Fatal("resume re-prompted after a consent whose check still fails")
	default:
	}
}

func TestResumeHintActivationAfterPermission_DropsConsentWhenSessionMovedOn(t *testing.T) {
	modal := newPermissionModal()
	shutdownCalled := make(chan struct{})

	core, logs := observer.New(zap.DebugLevel)

	handler := newVisionHintsHandler(modal.system, zap.New(core), func() { close(shutdownCalled) })

	handler.ActivateMode(modecmd.Activation{Mode: domain.ModeHints})

	modal.awaitOpen(t)

	// Move the mode session on while the dialog is open.
	handler.SetModeGrid()

	modal.consent <- ports.ScreenCaptureQuit

	deadline := time.Now().Add(2 * time.Second)
	for logs.FilterMessageSnippet("Dropping screen-capture consent").Len() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("stale consent was never dropped")
		}

		time.Sleep(2 * time.Millisecond)
	}

	select {
	case <-shutdownCalled:
		t.Fatal("stale quit consent reached shutdown")
	default:
	}
}
