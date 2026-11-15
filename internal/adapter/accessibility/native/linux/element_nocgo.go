//go:build linux && !cgo

package linux

func linuxFocusedApplicationIdentity() (string, int) {
	return "", 0
}
