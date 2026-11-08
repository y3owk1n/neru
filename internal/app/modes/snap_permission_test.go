package modes

import (
	"context"
	"image"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

func TestHandler_Snap_CaptureStrategyWithoutPermissionAsksOnceAndFails(t *testing.T) {
	modal := newPermissionModal()
	shutdownCalled := make(chan struct{})

	handler := newVisionHintsHandler(modal.system, zap.NewNop(), func() { close(shutdownCalled) })

	_, err := handler.Snap(context.Background(), SnapOptions{})
	if !derrors.IsCode(err, derrors.CodeActionFailed) {
		t.Fatalf("Snap() error = %v, want an action failure for missing permission", err)
	}

	modal.awaitOpen(t)

	if !handler.mu.TryLock() {
		t.Fatal("handler lock held while the permission dialog is open")
	}
	handler.mu.Unlock()

	// A held snap's repeat raises no second dialog. newPermissionModal closes
	// started on each request, so a second one would panic.
	_, err = handler.Snap(context.Background(), SnapOptions{})
	if !derrors.IsCode(err, derrors.CodeActionFailed) {
		t.Fatalf("second Snap() error = %v, want an action failure", err)
	}

	modal.consent <- ports.ScreenCaptureQuit

	select {
	case <-shutdownCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("quit consent never reached shutdown")
	}
}

func TestHandler_Snap_StrategyOverrideThatReadsTheScreenAsksForPermission(t *testing.T) {
	modal := newPermissionModal()
	handler := newCaptureHintsHandler(domain.StrategyAXTree, modal.system, zap.NewNop(), func() {})

	_, err := handler.Snap(context.Background(), SnapOptions{Strategy: domain.StrategySegment})
	if !derrors.IsCode(err, derrors.CodeActionFailed) {
		t.Fatalf("Snap() error = %v, want an action failure for missing permission", err)
	}

	modal.awaitOpen(t)

	modal.consent <- ports.ScreenCaptureCanceled
}

func TestHandler_Snap_AXTreeNeverAsksForScreenCapturePermission(t *testing.T) {
	var requests atomic.Int32

	systemMock := &portmocks.MockSystemPort{
		CheckScreenCapturePermissionFunc: func(context.Context) bool { return false },
		RequestScreenCapturePermissionFunc: func(context.Context) ports.ScreenCaptureConsent {
			requests.Add(1)

			return ports.ScreenCaptureCanceled
		},
	}

	handler := newCaptureHintsHandler(domain.StrategyAXTree, systemMock, zap.NewNop(), func() {})
	handler.hintService = services.NewHintService(
		&portmocks.MockAccessibilityPort{},
		&portmocks.MockOverlayPort{},
		systemMock,
		nil,
		configpkg.HintsConfig{Enabled: true, Strategy: domain.StrategyAXTree},
		zap.NewNop(),
		nil,
	)

	_, err := handler.Snap(context.Background(), SnapOptions{})
	if derrors.IsCode(err, derrors.CodeActionFailed) {
		t.Fatalf("Snap() error = %v, an axtree snap must not be refused for screen recording", err)
	}

	if got := requests.Load(); got != 0 {
		t.Errorf("permission dialog requested %d times, want 0 for axtree", got)
	}
}

func TestHandler_Snap_ScansWithTheStrategyItCheckedWhenTheAppLookupRecovers(t *testing.T) {
	var lookups, scans atomic.Int32

	// The first lookup fails, so the permission check sees the root axtree
	// strategy. A second lookup would find the app whose strategy captures.
	accessibility := &portmocks.MockAccessibilityPort{
		FocusedAppBundleIDFunc: func(context.Context) (string, error) {
			if lookups.Add(1) == 1 {
				return "", derrors.New(derrors.CodeAccessibilityFailed, "no focused app yet")
			}

			return "com.example.Captured", nil
		},
	}
	vision := &portmocks.MockVisionPort{
		DetectSegmentsFunc: func(
			context.Context, image.Rectangle, configpkg.HintsSegmentConfig,
		) ([]*element.Element, error) {
			scans.Add(1)

			return nil, nil
		},
	}
	systemMock := &portmocks.MockSystemPort{
		CheckScreenCapturePermissionFunc: func(context.Context) bool { return false },
		RequestScreenCapturePermissionFunc: func(context.Context) ports.ScreenCaptureConsent {
			return ports.ScreenCaptureCanceled
		},
	}
	hints := configpkg.HintsConfig{
		Enabled:  true,
		Strategy: domain.StrategyAXTree,
		AppConfigs: []configpkg.AppConfig{
			{BundleID: "com.example.Captured", Strategy: domain.StrategySegment},
		},
	}

	handler := newCaptureHintsHandler(domain.StrategyAXTree, systemMock, zap.NewNop(), func() {})
	handler.config.Hints = hints
	handler.actionService = services.NewActionService(
		accessibility, &portmocks.MockOverlayPort{}, systemMock, zap.NewNop(),
	)
	handler.hintService = services.NewHintService(
		accessibility, &portmocks.MockOverlayPort{}, systemMock, nil, hints, zap.NewNop(), vision,
	)

	_, _ = handler.Snap(context.Background(), SnapOptions{})

	if n := scans.Load(); n != 0 {
		t.Errorf("segment captured %d times after a check that saw axtree, want 0", n)
	}
}
