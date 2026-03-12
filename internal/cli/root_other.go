//go:build !darwin

package cli

// isRunningFromAppBundle always returns false on non-macOS platforms.
// App bundles (.app/Contents/MacOS) are a macOS-only concept.
func isRunningFromAppBundle() bool {
	return false
}
