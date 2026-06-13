//go:build linux && !cgo

package linux

import "github.com/y3owk1n/neru/internal/core/ports"

// NewFontResolver returns a fontconfig-less ports.FontResolver. It still
// maps generic aliases to the Linux baseline families so non-CGO builds
// behave deterministically, but cannot verify whether a user-supplied
// family is installed. CGO builds (the default) get a full fontconfig
// adapter in font_linux_cgo.go.
func NewFontResolver() ports.FontResolver {
	return &passthroughResolver{}
}

// passthroughResolver maps generic aliases to known-good installed
// families and otherwise returns the input unchanged.
type passthroughResolver struct{}

// Resolve implements ports.FontResolver.
func (passthroughResolver) Resolve(family string, _ bool) string {
	return mapGenericAlias(family)
}
