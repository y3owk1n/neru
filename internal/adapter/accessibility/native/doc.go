// Package native is the accessibility client built on each OS's own API. One
// shell (client.go, query.go) is written against Element, ElementInfo,
// TreeNode and TreeOptions; a build-tagged backend_<os>.go binds those names
// to the platform package supplying them — AXUIElement on darwin, UI
// Automation on windows, input/geometry on linux. Only the dispatch files
// name a platform.
package native
