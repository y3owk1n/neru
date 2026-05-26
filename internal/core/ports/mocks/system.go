package mocks

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// MockSystemPort is a mock implementation of ports.SystemPort.
type MockSystemPort struct {
	ConfigDirFunc                   func() (string, error)
	UserDataDirFunc                 func() (string, error)
	LogDirFunc                      func() (string, error)
	FocusedAppPIDFunc               func(ctx context.Context) (int, error)
	AppNameByPIDFunc                func(ctx context.Context, pid int) (string, error)
	AppBundleIDByPIDFunc            func(ctx context.Context, pid int) (string, error)
	ScreenBoundsFunc                func(ctx context.Context) (image.Rectangle, error)
	ScreenBoundsByNameFunc          func(ctx context.Context, name string) (image.Rectangle, bool, error)
	ScreenNamesFunc                 func(ctx context.Context) ([]string, error)
	FocusedWindowBoundsFunc         func(ctx context.Context) (image.Rectangle, bool, error)
	MoveCursorToPointFunc           func(ctx context.Context, point image.Point, bypassSmooth bool) error
	WaitForCursorIdleFunc           func(ctx context.Context) error
	CursorPositionFunc              func(ctx context.Context) (image.Point, error)
	CheckPermissionsFunc            func(ctx context.Context) error
	IsDarkModeFunc                  func() bool
	IsSecureInputEnabledFunc        func() bool
	ShowSecureInputNotificationFunc func()
	ShowAlertFunc                   func(ctx context.Context, title, message string) error
	ShowNotificationFunc            func(title, message string)
	CapabilitiesFunc                func() ports.PlatformCapabilities
	HealthFunc                      func(ctx context.Context) error
}

// ConfigDir is a mock implementation.
func (m *MockSystemPort) ConfigDir() (string, error) {
	if m.ConfigDirFunc != nil {
		return m.ConfigDirFunc()
	}

	return "", nil
}

// UserDataDir is a mock implementation.
func (m *MockSystemPort) UserDataDir() (string, error) {
	if m.UserDataDirFunc != nil {
		return m.UserDataDirFunc()
	}

	return "", nil
}

// LogDir is a mock implementation.
func (m *MockSystemPort) LogDir() (string, error) {
	if m.LogDirFunc != nil {
		return m.LogDirFunc()
	}

	return "", nil
}

// FocusedApplicationPID is a mock implementation.
func (m *MockSystemPort) FocusedApplicationPID(ctx context.Context) (int, error) {
	if m.FocusedAppPIDFunc != nil {
		return m.FocusedAppPIDFunc(ctx)
	}

	return 0, nil
}

// ApplicationNameByPID is a mock implementation.
func (m *MockSystemPort) ApplicationNameByPID(ctx context.Context, pid int) (string, error) {
	if m.AppNameByPIDFunc != nil {
		return m.AppNameByPIDFunc(ctx, pid)
	}

	return "", nil
}

// ApplicationBundleIDByPID is a mock implementation.
func (m *MockSystemPort) ApplicationBundleIDByPID(ctx context.Context, pid int) (string, error) {
	if m.AppBundleIDByPIDFunc != nil {
		return m.AppBundleIDByPIDFunc(ctx, pid)
	}

	return "", nil
}

// ScreenBounds is a mock implementation.
func (m *MockSystemPort) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	if m.ScreenBoundsFunc != nil {
		return m.ScreenBoundsFunc(ctx)
	}

	return image.Rectangle{}, nil
}

// ScreenBoundsByName is a mock implementation.
func (m *MockSystemPort) ScreenBoundsByName(
	ctx context.Context,
	name string,
) (image.Rectangle, bool, error) {
	if m.ScreenBoundsByNameFunc != nil {
		return m.ScreenBoundsByNameFunc(ctx, name)
	}

	return image.Rectangle{}, false, nil
}

// ScreenNames is a mock implementation.
func (m *MockSystemPort) ScreenNames(ctx context.Context) ([]string, error) {
	if m.ScreenNamesFunc != nil {
		return m.ScreenNamesFunc(ctx)
	}

	return nil, nil
}

// FocusedWindowBounds is a mock implementation.
func (m *MockSystemPort) FocusedWindowBounds(
	ctx context.Context,
) (image.Rectangle, bool, error) {
	if m.FocusedWindowBoundsFunc != nil {
		return m.FocusedWindowBoundsFunc(ctx)
	}

	return image.Rectangle{}, false, nil
}

// MoveCursorToPoint is a mock implementation.
func (m *MockSystemPort) MoveCursorToPoint(
	ctx context.Context,
	point image.Point,
	bypassSmooth bool,
) error {
	if m.MoveCursorToPointFunc != nil {
		return m.MoveCursorToPointFunc(ctx, point, bypassSmooth)
	}

	return nil
}

// WaitForCursorIdle is a mock implementation.
func (m *MockSystemPort) WaitForCursorIdle(ctx context.Context) error {
	if m.WaitForCursorIdleFunc != nil {
		return m.WaitForCursorIdleFunc(ctx)
	}

	return nil
}

// CursorPosition is a mock implementation.
func (m *MockSystemPort) CursorPosition(ctx context.Context) (image.Point, error) {
	if m.CursorPositionFunc != nil {
		return m.CursorPositionFunc(ctx)
	}

	return image.Point{}, nil
}

// CheckPermissions is a mock implementation.
func (m *MockSystemPort) CheckPermissions(ctx context.Context) error {
	if m.CheckPermissionsFunc != nil {
		return m.CheckPermissionsFunc(ctx)
	}

	return nil
}

// IsDarkMode is a mock implementation.
func (m *MockSystemPort) IsDarkMode() bool {
	if m.IsDarkModeFunc != nil {
		return m.IsDarkModeFunc()
	}

	return false
}

// IsSecureInputEnabled is a mock implementation.
func (m *MockSystemPort) IsSecureInputEnabled() bool {
	if m.IsSecureInputEnabledFunc != nil {
		return m.IsSecureInputEnabledFunc()
	}

	return false
}

// ShowSecureInputNotification is a mock implementation.
func (m *MockSystemPort) ShowSecureInputNotification() {
	if m.ShowSecureInputNotificationFunc != nil {
		m.ShowSecureInputNotificationFunc()
	}
}

// ShowAlert is a mock implementation.
func (m *MockSystemPort) ShowAlert(ctx context.Context, title, message string) error {
	if m.ShowAlertFunc != nil {
		return m.ShowAlertFunc(ctx, title, message)
	}

	return nil
}

// ShowNotification is a mock implementation.
func (m *MockSystemPort) ShowNotification(title, message string) {
	if m.ShowNotificationFunc != nil {
		m.ShowNotificationFunc(title, message)
	}
}

// Capabilities is a mock implementation.
func (m *MockSystemPort) Capabilities() ports.PlatformCapabilities {
	if m.CapabilitiesFunc != nil {
		return m.CapabilitiesFunc()
	}

	return ports.PlatformCapabilities{}
}

// Health is a mock implementation.
func (m *MockSystemPort) Health(ctx context.Context) error {
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}

	return nil
}

var _ ports.SystemPort = (*MockSystemPort)(nil)
