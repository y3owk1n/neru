package accessibility

import (
	"image"

	"github.com/y3owk1n/neru/internal/core/domain/action"
)

// MockAXClient is a mock implementation of AXClient for testing.
type MockAXClient struct {
	MockFrontmostWindow    AXWindow
	MockFrontmostWindowErr error

	MockFocusedApp    AXApp
	MockFocusedAppErr error

	MockClickableNodes    []AXNode
	MockClickableNodesErr error

	MockMenuBarNodes    []AXNode
	MockMenuBarNodesErr error

	MockBundleNodes    []AXNode
	MockBundleNodesErr error

	MockScreenBounds image.Rectangle

	MockActionErr error
	MockScrollErr error

	MockPermissions bool

	MockClickableRoles []string

	MockMissionControlActive bool

	LastCalledBundleID         string
	LastClickableNodesRoles    []string
	LastBundleRoles            []string
	ClickableNodesRolesHistory [][]string
}

// FrontmostWindow returns the configured frontmost window or error.
func (m *MockAXClient) FrontmostWindow() (AXWindow, error) {
	return m.MockFrontmostWindow, m.MockFrontmostWindowErr
}

// FocusedApplication returns the configured focused application or error.
func (m *MockAXClient) FocusedApplication() (AXApp, error) {
	return m.MockFocusedApp, m.MockFocusedAppErr
}

// ApplicationByBundleID returns the configured application by bundle ID or error.
func (m *MockAXClient) ApplicationByBundleID(_ string) (AXApp, error) {
	return m.MockFocusedApp, m.MockFocusedAppErr // Reuse focused app for simplicity or add specific field
}

// ClickableNodes returns the configured clickable nodes or error.
func (m *MockAXClient) ClickableNodes(_ AXElement, _ bool, roles []string) ([]AXNode, error) {
	m.LastClickableNodesRoles = roles
	m.ClickableNodesRolesHistory = append(m.ClickableNodesRolesHistory, roles)

	return m.MockClickableNodes, m.MockClickableNodesErr
}

// MenuBarClickableElements returns the configured menu bar nodes or error.
func (m *MockAXClient) MenuBarClickableElements() ([]AXNode, error) {
	return m.MockMenuBarNodes, m.MockMenuBarNodesErr
}

// ClickableElementsFromBundleID returns the configured nodes for bundle ID or error.
func (m *MockAXClient) ClickableElementsFromBundleID(
	bundleID string,
	roles []string,
) ([]AXNode, error) {
	m.LastCalledBundleID = bundleID
	m.LastBundleRoles = roles

	return m.MockBundleNodes, m.MockBundleNodesErr
}

// ActiveScreenBounds returns the configured screen bounds.
func (m *MockAXClient) ActiveScreenBounds() image.Rectangle {
	return m.MockScreenBounds
}

// PerformAction returns the configured action error.
func (m *MockAXClient) PerformAction(_ action.Type, _ image.Point, _ bool) error {
	return m.MockActionErr
}

// Scroll returns the configured scroll error.
func (m *MockAXClient) Scroll(_, _ int) error {
	return m.MockScrollErr
}

// MoveMouse is a no-op mock implementation.
func (m *MockAXClient) MoveMouse(_ image.Point, _ bool) {
	// No-op
}

// CursorPosition returns the zero point.
func (m *MockAXClient) CursorPosition() image.Point {
	return image.Point{}
}

// CheckPermissions returns the configured permissions state.
func (m *MockAXClient) CheckPermissions() bool {
	return m.MockPermissions
}

// SetClickableRoles updates the configured clickable roles.
func (m *MockAXClient) SetClickableRoles(roles []string) {
	m.MockClickableRoles = roles
}

// ClickableRoles returns the configured clickable roles.
func (m *MockAXClient) ClickableRoles() []string {
	return m.MockClickableRoles
}

// IsMissionControlActive returns the configured Mission Control state.
func (m *MockAXClient) IsMissionControlActive() bool {
	return m.MockMissionControlActive
}

// Mock implementations for Window, App, Node

// MockWindow is a mock implementation of AXWindow.
type MockWindow struct{}

// Release is a no-op.
func (w *MockWindow) Release() {}

// MockApp is a mock implementation of AXApp.
type MockApp struct {
	MockBundleID string
	MockInfo     *AXAppInfo
}

// Release is a no-op.
func (a *MockApp) Release() {}

// BundleIdentifier returns the configured bundle ID.
func (a *MockApp) BundleIdentifier() string {
	return a.MockBundleID
}

// Info returns the configured app info.
func (a *MockApp) Info() (*AXAppInfo, error) {
	return a.MockInfo, nil
}

// MockNode is a mock implementation of AXNode.
type MockNode struct {
	MockID          string
	MockBounds      image.Rectangle
	MockRole        string
	MockTitle       string
	MockDescription string
	MockClickable   bool
}

// ID returns the configured ID.
func (n *MockNode) ID() string {
	return n.MockID
}

// Bounds returns the configured bounds.
func (n *MockNode) Bounds() image.Rectangle {
	return n.MockBounds
}

// Role returns the configured role.
func (n *MockNode) Role() string {
	return n.MockRole
}

// Title returns the configured title.
func (n *MockNode) Title() string {
	return n.MockTitle
}

// Description returns the configured description.
func (n *MockNode) Description() string {
	return n.MockDescription
}

// IsClickable returns the configured clickable state.
func (n *MockNode) IsClickable() bool {
	return n.MockClickable
}
