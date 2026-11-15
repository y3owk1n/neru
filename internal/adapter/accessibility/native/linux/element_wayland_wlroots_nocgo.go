//go:build linux && !cgo

package linux

func wlrootsFocusedApplicationIdentity() (string, int) { return "", 0 }
