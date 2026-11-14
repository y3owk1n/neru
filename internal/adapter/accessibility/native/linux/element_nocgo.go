//go:build linux && !cgo

package linux

func linuxFocusedApplicationIdentity() (string, int) {
	return "", 0
}

func linuxApplicationBundleIdentifier(pid int) string {
	_ = pid
	return ""
}
