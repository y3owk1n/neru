// Package accessibility implements ports.AccessibilityPort, providing access
// to platform UI elements (clickable nodes, focused app, element trees).
//
// On macOS the implementation uses the AXUIElement / CGo bridge. On Linux
// the target is AT-SPI; on Windows, UI Automation. Platform-specific code
// lives in element_darwin.go / element_linux.go / element_windows.go and
// tree.go (darwin) / tree_linux.go / tree_windows.go. The adapter.go and
// infra_client.go files are platform-agnostic.
package accessibility
