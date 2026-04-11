//go:build linux && !cgo

package accessibility

func linuxFocusedApplicationIdentity() (string, int) {
	return "", 0
}

func linuxApplicationBundleIdentifier(pid int) string {
	_ = pid
	return ""
}
