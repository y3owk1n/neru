// Package native is the accessibility client built on each OS's own API:
// AXUIElement on macOS, UI Automation on Windows, and on Linux the input and
// geometry half that package atspi delegates to.
//
// One shell (client.go, query.go) is specialised per platform by the
// build-tagged Element, ElementInfo, TreeNode and TreeOptions types that
// element_*.go and tree_*.go declare. That is why macOS and Windows are not yet
// separate packages: they are not separate implementations, they are the same
// implementation over different types. Splitting them means parameterising the
// shell over an interface first — see docs/RESTRUCTURE.md.
package native
