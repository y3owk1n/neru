package accessibility

import (
	"image"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// MockAXClient is a mock implementation of AXClient for testing.
type MockAXClient struct {
	FrontmostWindow    AXWindow
	FrontmostWindowErr error

	FocusedApp    AXApp
	FocusedAppErr error

	ClickableNodes    []AXNode
	ClickableNodesErr error

	MenuBarNodes    []AXNode
	MenuBarNodesErr error

	BundleNodes    []AXNode
	BundleNodesErr error

	ScreenBounds image.Rectangle

	ActionErr error
	ScrollErr error

	Permissions bool

	ClickableRoles []string

	MissionControlActive bool
}

// GetFrontmostWindow returns the configured frontmost window or error.
func (m *MockAXClient) GetFrontmostWindow() (AXWindow, error) {
	return m.FrontmostWindow, m.FrontmostWindowErr
}

// GetFocusedApplication returns the configured focused application or error.
func (m *MockAXClient) GetFocusedApplication() (AXApp, error) {
	return m.FocusedApp, m.FocusedAppErr
}

// GetApplicationByBundleID returns the configured application by bundle ID or error.
func (m *MockAXClient) GetApplicationByBundleID(_ string) (AXApp, error) {
	return m.FocusedApp, m.FocusedAppErr // Reuse focused app for simplicity or add specific field
}

// GetClickableNodes returns the configured clickable nodes or error.
func (m *MockAXClient) GetClickableNodes(_ AXElement, _ bool) ([]AXNode, error) {
	return m.ClickableNodes, m.ClickableNodesErr
}

// GetMenuBarClickableElements returns the configured menu bar nodes or error.
func (m *MockAXClient) GetMenuBarClickableElements() ([]AXNode, error) {
	return m.MenuBarNodes, m.MenuBarNodesErr
}

// GetClickableElementsFromBundleID returns the configured nodes for bundle ID or error.
func (m *MockAXClient) GetClickableElementsFromBundleID(_ string) ([]AXNode, error) {
	return m.BundleNodes, m.BundleNodesErr
}

// GetActiveScreenBounds returns the configured screen bounds.
func (m *MockAXClient) GetActiveScreenBounds() image.Rectangle {
	return m.ScreenBounds
}

// PerformAction returns the configured action error.
func (m *MockAXClient) PerformAction(_ action.Type, _ image.Point, _ bool) error {
	return m.ActionErr
}

// Scroll returns the configured scroll error.
func (m *MockAXClient) Scroll(_, _ int) error {
	return m.ScrollErr
}

// MoveMouse is a no-op mock implementation.
func (m *MockAXClient) MoveMouse(_ image.Point) {
	// No-op
}

// GetCursorPosition returns the zero point.
func (m *MockAXClient) GetCursorPosition() image.Point {
	return image.Point{}
}

// CheckPermissions returns the configured permissions state.
func (m *MockAXClient) CheckPermissions() bool {
	return m.Permissions
}

// SetClickableRoles updates the configured clickable roles.
func (m *MockAXClient) SetClickableRoles(roles []string) {
	m.ClickableRoles = roles
}

// GetClickableRoles returns the configured clickable roles.
func (m *MockAXClient) GetClickableRoles() []string {
	return m.ClickableRoles
}

// IsMissionControlActive returns the configured Mission Control state.
func (m *MockAXClient) IsMissionControlActive() bool {
	return m.MissionControlActive
}

// Mock implementations for Window, App, Node

// MockWindow is a mock implementation of AXWindow.
type MockWindow struct{}

// Release is a no-op.
func (w *MockWindow) Release() {}

// MockApp is a mock implementation of AXApp.
type MockApp struct {
	BundleID string
	Info     *AXAppInfo
}

// Release is a no-op.
func (a *MockApp) Release() {}

// GetBundleIdentifier returns the configured bundle ID.
func (a *MockApp) GetBundleIdentifier() string {
	return a.BundleID
}

// GetInfo returns the configured app info.
func (a *MockApp) GetInfo() (*AXAppInfo, error) {
	return a.Info, nil
}

// MockNode is a mock implementation of AXNode.
type MockNode struct {
	ID          string
	Bounds      image.Rectangle
	Role        string
	Title       string
	Description string
	Clickable   bool
}

// GetID returns the configured ID.
func (n *MockNode) GetID() string {
	return n.ID
}

// GetBounds returns the configured bounds.
func (n *MockNode) GetBounds() image.Rectangle {
	return n.Bounds
}

// GetRole returns the configured role.
func (n *MockNode) GetRole() string {
	return n.Role
}

// GetTitle returns the configured title.
func (n *MockNode) GetTitle() string {
	return n.Title
}

// GetDescription returns the configured description.
func (n *MockNode) GetDescription() string {
	return n.Description
}

// IsClickable returns the configured clickable state.
func (n *MockNode) IsClickable() bool {
	return n.Clickable
}
