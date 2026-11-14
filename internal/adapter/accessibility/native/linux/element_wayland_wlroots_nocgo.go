//go:build linux && !cgo

package linux

func wlrootsFocusedApplicationIdentity() (string, int) { return "", 0 }

func wlrootsApplicationBundleIdentifier(pid int) string {
	_ = pid
	return ""
}
