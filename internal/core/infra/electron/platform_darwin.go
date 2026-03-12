//go:build darwin

package electron

import "github.com/y3owk1n/neru/internal/core/infra/platform/darwin"

func platformSetApplicationAttribute(pid int, attribute string, value bool) bool {
	return darwin.SetApplicationAttribute(pid, attribute, value)
}
