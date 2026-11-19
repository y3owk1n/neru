package accessibility_test

import (
	"context"
	"slices"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/accessibility"
	"github.com/y3owk1n/neru/internal/adapter/accessibility/ax"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	bundleIDAppleDock    = "com.apple.dock"
	bundleIDAppleFinder  = "com.apple.finder"
	bundleIDGoogleChrome = "com.google.Chrome"
	axButtonRole         = "AXButton"
	axLinkRole           = "AXLink"
	axApplicationRole    = "AXApplication"
)

// errTestAccessibility is a static error for testing accessibility failures.
var errTestAccessibility = derrors.New(derrors.CodeAccessibilityDenied, "accessibility denied")

func TestAdapter_UpdateClickableRoles(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, mockClient, false)

	newRoles := []string{axButtonRole, axLinkRole, "AXMenuItem"}
	adapter.UpdateClickableRoles(newRoles)

	if len(mockClient.MockClickableRoles) != len(newRoles) {
		t.Errorf(
			"Expected mock to have %d roles, got %d",
			len(newRoles),
			len(mockClient.MockClickableRoles),
		)
	}
}

// mockAXApp is a mock implementation of ax.App for testing.
type mockAXApp struct {
	bundleID string
	MockInfo *ax.AppInfo
}

func (m *mockAXApp) Release()                 {}
func (m *mockAXApp) BundleIdentifier() string { return m.bundleID }

func (m *mockAXApp) Info() (*ax.AppInfo, error) { return m.MockInfo, nil }

func TestAdapter_FocusedAppBundleID(t *testing.T) {
	tests := []struct {
		name       string
		mockApp    *mockAXApp
		mockErr    error
		wantBundle string
		wantErr    bool
	}{
		{
			name:       "success",
			mockApp:    &mockAXApp{bundleID: bundleIDGoogleChrome},
			mockErr:    nil,
			wantBundle: bundleIDGoogleChrome,
			wantErr:    false,
		},
		{
			name:       "error from client",
			mockApp:    nil,
			mockErr:    errTestAccessibility,
			wantBundle: "",
			wantErr:    true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			logger := zap.NewNop()
			mockClient := &accessibility.MockAXClient{
				MockFocusedApp:    testCase.mockApp,
				MockFocusedAppErr: testCase.mockErr,
			}
			adapter := accessibility.NewAdapter(logger, mockClient, false)
			ctx := context.Background()

			bundleID, bundleIDErr := adapter.FocusedAppBundleID(ctx)

			if (bundleIDErr != nil) != testCase.wantErr {
				t.Errorf(
					"FocusedAppBundleID() error = %v, wantErr %v",
					bundleIDErr,
					testCase.wantErr,
				)

				return
			}

			if bundleID != testCase.wantBundle {
				t.Errorf("FocusedAppBundleID() = %v, want %v", bundleID, testCase.wantBundle)
			}
		})
	}
}

func TestAdapter_Health(t *testing.T) {
	tests := []struct {
		name        string
		permissions bool
		wantErr     bool
	}{
		{name: "permissions granted", permissions: true, wantErr: false},
		{name: "permissions denied", permissions: false, wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockClient := &accessibility.MockAXClient{MockPermissions: testCase.permissions}
			adapter := accessibility.NewAdapter(zap.NewNop(), mockClient, false)

			healthErr := adapter.Health(context.Background())
			if (healthErr != nil) != testCase.wantErr {
				t.Errorf("Health() error = %v, wantErr %v", healthErr, testCase.wantErr)
			}
		})
	}
}

func TestAdapter_RolePassing(t *testing.T) {
	logger := zap.NewNop()
	mockWindow := &accessibility.MockWindow{}
	mockClient := &accessibility.MockAXClient{
		MockPermissions:     true,
		MockFrontmostWindow: mockWindow,
	}

	initialRoles := []string{axButtonRole}
	mockClient.SetClickableRoles(initialRoles) // Initialize mock state

	adapter := accessibility.NewAdapter(logger, mockClient, false)
	ctx := context.Background()

	t.Run("Frontmost Window Uses Filter Roles", func(t *testing.T) {
		mockClient.LastClickableNodesRoles = nil
		mockClient.ClickableNodesRolesHistory = nil

		filter := ports.ElementFilter{
			Roles: []element.Role{element.RoleButton, element.Role("AXTabGroup")},
		}

		_, _ = adapter.ClickableElements(ctx, filter)

		if !slices.Equal(mockClient.LastClickableNodesRoles, []string{axButtonRole, "AXTabGroup"}) {
			t.Errorf(
				"Expected LastClickableNodesRoles to use filter roles, got: %v",
				mockClient.LastClickableNodesRoles,
			)
		}
	})

	t.Run("Additional Menubar Targets", func(t *testing.T) {
		filter := ports.ElementFilter{
			IncludeMenubar:           true,
			AdditionalMenubarTargets: []string{"com.example.app"},
		}

		_, _ = adapter.ClickableElements(ctx, filter)

		calledRoles := mockClient.LastBundleRoles

		hasMenuBarItem := slices.Contains(calledRoles, "AXMenuBarItem")

		if !hasMenuBarItem {
			t.Errorf("Expected LastBundleRoles to contain AXMenuBarItem, got: %v", calledRoles)
		}

		// Verify Global roles were NOT changed via SetClickableRoles calls during execution
		// The mock implementation of SetClickableRoles updates MockClickableRoles
		if len(mockClient.MockClickableRoles) != len(initialRoles) {
			t.Errorf(
				"Global roles changed! Expected %d, got %d",
				len(initialRoles),
				len(mockClient.MockClickableRoles),
			)
		}
	})

	t.Run("Dock Elements", func(t *testing.T) {
		mockClient.LastClickableNodesRoles = nil
		mockClient.ClickableNodesRolesHistory = nil

		// The mock returns MockFocusedApp from ApplicationByBundleID too, and the
		// adapter only reads the Dock when its Info() role is AXApplication.
		mockClient.MockFocusedApp = &mockAXApp{
			bundleID: bundleIDAppleDock,
			MockInfo: &ax.AppInfo{
				Role:  axApplicationRole,
				Title: "Dock",
			},
		}

		filter := ports.ElementFilter{
			IncludeDock: true,
		}

		_, _ = adapter.ClickableElements(ctx, filter)

		found := false
		for _, roles := range mockClient.ClickableNodesRolesHistory {
			if slices.Contains(roles, "AXDockItem") {
				found = true

				break
			}
		}

		if !found {
			t.Errorf(
				"Expected ClickableNodesRolesHistory to contain roles with AXDockItem, got: %v",
				mockClient.ClickableNodesRolesHistory,
			)
		}
	})

	t.Run("Picture in Picture Elements", func(t *testing.T) {
		mockClient.LastCalledBundleID = ""
		mockClient.LastBundleRoles = nil

		filter := ports.ElementFilter{
			IncludePIP: true,
		}

		_, _ = adapter.ClickableElements(ctx, filter)

		if mockClient.LastCalledBundleID != "com.apple.PIPAgent" {
			t.Errorf(
				"Expected PIP supplementary query to use com.apple.PIPAgent, got: %q",
				mockClient.LastCalledBundleID,
			)
		}

		if mockClient.LastBundleRoles != nil {
			t.Errorf(
				"Expected PIP query to pass nil roles (no role restriction), got: %v",
				mockClient.LastBundleRoles,
			)
		}
	})

	t.Run("Screen Capture Elements", func(t *testing.T) {
		mockClient.LastCalledBundleID = ""
		mockClient.LastBundleRoles = nil

		filter := ports.ElementFilter{
			IncludeScreenCapture: true,
		}

		_, _ = adapter.ClickableElements(ctx, filter)

		if mockClient.LastCalledBundleID != "com.apple.screencaptureui" {
			t.Errorf(
				"Expected Screen Capture supplementary query to use com.apple.screencaptureui, got: %q",
				mockClient.LastCalledBundleID,
			)
		}

		if mockClient.LastBundleRoles != nil {
			t.Errorf(
				"Expected Screen Capture query to pass nil roles (no role restriction), got: %v",
				mockClient.LastBundleRoles,
			)
		}
	})
}

// TestAdapter_ClickableElements_MissionControlIsPartOfDockHints pins when
// Mission Control's own elements are collected: while it is up and Dock hints
// are asked for, which is what include_dock_hints promises, and at no other
// time.
func TestAdapter_ClickableElements_MissionControlIsPartOfDockHints(t *testing.T) {
	tests := []struct {
		name           string
		missionControl bool
		includeDock    bool
		wantCollected  bool
	}{
		{
			name:           "mission control with dock hints",
			missionControl: true,
			includeDock:    true,
			wantCollected:  true,
		},
		{name: "mission control without dock hints", missionControl: true, includeDock: false},
		{name: "dock hints outside mission control", missionControl: false, includeDock: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockWindow := &accessibility.MockWindow{}
			mockClient := &accessibility.MockAXClient{
				MockPermissions:          true,
				MockFrontmostWindow:      mockWindow,
				MockMissionControlActive: testCase.missionControl,
				MockFocusedApp: &mockAXApp{
					bundleID: bundleIDAppleDock,
					MockInfo: &ax.AppInfo{Role: axApplicationRole},
				},
			}

			adapter := accessibility.NewAdapter(zap.NewNop(), mockClient, true)

			_, err := adapter.ClickableElements(
				context.Background(),
				ports.ElementFilter{IncludeDock: testCase.includeDock},
			)
			if err != nil {
				t.Fatalf("ClickableElements() error = %v", err)
			}

			if collected := mockClient.MissionControlCalls > 0; collected != testCase.wantCollected {
				t.Errorf(
					"Mission Control collected = %v, want %v",
					collected,
					testCase.wantCollected,
				)
			}
		})
	}
}

// TestAdapter_UpdateMissionControlDetection_FollowsTheReloadedSetting covers
// hints.detect_mission_control changing on reload. The adapter used to keep
// the value it was built with, so the change only took effect after a restart.
func TestAdapter_UpdateMissionControlDetection_FollowsTheReloadedSetting(t *testing.T) {
	tests := []struct {
		name          string
		atStartup     bool
		reloadedTo    bool
		wantCollected bool
	}{
		{name: "switched on by a reload", atStartup: false, reloadedTo: true, wantCollected: true},
		{
			name:          "switched off by a reload",
			atStartup:     true,
			reloadedTo:    false,
			wantCollected: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockWindow := &accessibility.MockWindow{}
			mockClient := &accessibility.MockAXClient{
				MockPermissions:          true,
				MockFrontmostWindow:      mockWindow,
				MockMissionControlActive: true,
				MockFocusedApp: &mockAXApp{
					bundleID: bundleIDAppleDock,
					MockInfo: &ax.AppInfo{Role: axApplicationRole},
				},
			}

			adapter := accessibility.NewAdapter(zap.NewNop(), mockClient, testCase.atStartup)
			adapter.UpdateMissionControlDetection(testCase.reloadedTo)

			_, err := adapter.ClickableElements(
				context.Background(),
				ports.ElementFilter{IncludeDock: true},
			)
			if err != nil {
				t.Fatalf("ClickableElements() error = %v", err)
			}

			if collected := mockClient.MissionControlCalls > 0; collected != testCase.wantCollected {
				t.Errorf(
					"Mission Control collected = %v, want %v",
					collected,
					testCase.wantCollected,
				)
			}
		})
	}
}
