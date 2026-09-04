//go:build linux && !cgo

package linux

import "github.com/y3owk1n/neru/internal/ports"

// NewFontResolver returns a fontconfig-less ports.FontResolver. It still
// maps generic aliases to the Linux baseline families so non-CGO builds
// behave deterministically, and returns every other family as written — the
// same answer the fontconfig build gives for a family that is installed. What
// it cannot do is see that a family is missing, so nothing falls back to the
// generic here; Cairo substitutes when the text is drawn, as NSFont does on
// macOS. CGO builds (the default) get the fontconfig adapter in
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
