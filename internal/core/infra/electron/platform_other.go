//go:build !darwin

package electron

func platformSetApplicationAttribute(_ int, _ string, _ bool) bool { return false }
