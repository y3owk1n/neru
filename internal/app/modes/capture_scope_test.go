package modes

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// newCaptureScopeHandler is a handler whose platform answers the focused-app
// query with bundleID or bundleErr, counting how often it is asked.
// windowedBundle is the app whose entry asks for the window.
const windowedBundle = "com.example.Windowed"

func newCaptureScopeHandler(bundleID string, bundleErr error, asked *int) *handlerState {
	return &handlerState{
		ctx:        context.Background(),
		logger:     zap.NewNop(),
		focusedApp: &focusedAppCell{},
		actionService: services.NewActionService(
			&portmocks.MockAccessibilityPort{
				FocusedAppBundleIDFunc: func(context.Context) (string, error) {
					*asked++

					return bundleID, bundleErr
				},
			},
			&portmocks.MockOverlayPort{},
			&portmocks.MockSystemPort{},
			zap.NewNop(),
		),
	}
}

func TestResolveCaptureScope_FlagThenAppThenSection(t *testing.T) {
	window := domain.CaptureScopeWindow
	withOverride := config.GridConfig{
		CaptureScope: domain.CaptureScopeScreen,
		AppConfigs: []config.AppConfig{
			{BundleID: windowedBundle, CaptureScope: domain.CaptureScopeWindow},
		},
	}

	tests := []struct {
		name       string
		section    config.GridConfig
		activation modecmd.Activation
		bundleID   string
		bundleErr  error
		published  string
		want       string
		wantAsked  int
	}{
		{
			name:    "the section alone",
			section: config.GridConfig{CaptureScope: domain.CaptureScopeScreen},
			want:    domain.CaptureScopeScreen,
		},
		{
			name:      "the focused app's entry shadows the section",
			section:   withOverride,
			bundleID:  windowedBundle,
			want:      domain.CaptureScopeWindow,
			wantAsked: 1,
		},
		{
			name:      "an app without an entry gets the section's",
			section:   withOverride,
			bundleID:  "com.example.Other",
			want:      domain.CaptureScopeScreen,
			wantAsked: 1,
		},
		{
			name:      "a failed app lookup leaves the section's in force",
			section:   withOverride,
			bundleErr: derrors.New(derrors.CodeNotSupported, "no focused app"),
			want:      domain.CaptureScopeScreen,
			wantAsked: 1,
		},
		{
			name:      "a published app is read and the platform is not asked",
			section:   withOverride,
			bundleID:  "com.example.Other",
			published: windowedBundle,
			want:      domain.CaptureScopeWindow,
		},
		{
			name:       "the flag shadows everything",
			section:    config.GridConfig{CaptureScope: domain.CaptureScopeScreen},
			activation: modecmd.Activation{CaptureScope: &window},
			want:       domain.CaptureScopeWindow,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			asked := 0
			handler := newCaptureScopeHandler(test.bundleID, test.bundleErr, &asked)

			if test.published != "" {
				handler.focusedApp.publish(test.published)
			}

			got := handler.resolveCaptureScope(domain.ModeNameGrid, test.activation, &test.section)
			if got != test.want {
				t.Fatalf("scope = %q, want %q", got, test.want)
			}

			if asked != test.wantAsked {
				t.Fatalf("the platform was asked %d times, want %d", asked, test.wantAsked)
			}
		})
	}
}

func TestCaptureStart_WindowScopeUsesTheFocusedWindowClippedToTheScreen(t *testing.T) {
	screen := image.Rect(0, 0, 1000, 800)

	tests := []struct {
		name    string
		scope   string
		window  image.Rectangle
		focused bool
		err     error
		want    image.Rectangle
	}{
		{
			name:  "the screen scope ignores the window",
			scope: domain.CaptureScopeScreen, window: image.Rect(100, 100, 500, 500),
			focused: true, want: screen,
		},
		{
			name:  "the focused window, where it is",
			scope: domain.CaptureScopeWindow, window: image.Rect(100, 100, 500, 500),
			focused: true, want: image.Rect(100, 100, 500, 500),
		},
		{
			name:  "a window off the edge is cut to the screen",
			scope: domain.CaptureScopeWindow, window: image.Rect(800, 600, 1400, 1000),
			focused: true, want: image.Rect(800, 600, 1000, 800),
		},
		{
			name:  "nothing focused falls back to the screen",
			scope: domain.CaptureScopeWindow, focused: false, want: screen,
		},
		{
			name:  "a platform without a window source falls back to the screen",
			scope: domain.CaptureScopeWindow, focused: true,
			err:  derrors.New(derrors.CodeNotSupported, "no window source"),
			want: screen,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := &handlerState{
				ctx:    context.Background(),
				logger: zap.NewNop(),
				system: &portmocks.MockSystemPort{
					FocusedWindowBoundsFunc: func(context.Context) (image.Rectangle, bool, error) {
						return test.window, test.focused, test.err
					},
				},
			}

			got := handler.captureStart(domain.ModeNameGrid, screen, test.scope)
			if !got.Eq(test.want) {
				t.Fatalf("start region = %v, want %v", got, test.want)
			}
		})
	}
}
