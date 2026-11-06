//go:build linux && !cgo

package linux

import "github.com/y3owk1n/neru/internal/ports"

// NewFontResolver returns a fontconfig-less ports.FontResolver. It maps generic
// aliases to the Linux baseline families and returns every other family as
// written. It cannot see that a family is missing, so Cairo substitutes at draw
// time, as NSFont does on macOS. CGO builds get the fontconfig adapter in
// font_cgo.go.
func NewFontResolver() ports.FontResolver {
	return &passthroughResolver{}
}

// passthroughResolver maps generic aliases to known-good installed
// families and otherwise returns the input unchanged.
type passthroughResolver struct{}

// Resolve implements ports.FontResolver.
func (passthroughResolver) Resolve(family string) string {
	return linuxFamilies.Resolve(family)
}
