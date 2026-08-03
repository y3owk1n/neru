// Package native is the accessibility client built on each OS's own API.
//
// One shell — client.go and query.go — is written against the Element,
// ElementInfo, TreeNode and TreeOptions types, and a build-tagged backend_<os>.go
// binds those names to the platform package that supplies them:
//
//   - darwin   AXUIElement
//   - windows  UI Automation over COM
//   - linux    input injection and window geometry, which package atspi
//     delegates to for everything that is not tree walking
//
// The shell holds no platform knowledge of its own; the dispatch files are the
// only place a platform is named.
package native
