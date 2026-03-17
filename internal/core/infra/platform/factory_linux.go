//go:build linux

package platform

import (
	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// NewSystemPort returns a Linux SystemPort implementation.
func NewSystemPort() (ports.SystemPort, error) {
	return linux.NewSystemAdapter(), nil
}

// ShowConfigOnboardingAlert is a stub on Linux.
func ShowConfigOnboardingAlert(_ string) int {
	return 0
}
