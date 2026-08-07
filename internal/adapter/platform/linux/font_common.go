//go:build linux

package linux

import "github.com/y3owk1n/neru/internal/adapter/platform/fontgeneric"

const (
	// defaultLinuxSans is the baseline Linux sans-serif family used when
	// fontconfig cannot be consulted (fontconfig absent, family missing,
	// generic alias requested).
	defaultLinuxSans = "DejaVu Sans"
	// defaultLinuxMono is the baseline Linux monospace family.
	defaultLinuxMono = "DejaVu Sans Mono"
	// defaultLinuxSerif is the baseline Linux serif family.
	defaultLinuxSerif = "DejaVu Serif"
)

// linuxFamilies is what the generic aliases mean on Linux.
var linuxFamilies = fontgeneric.Families{
	Sans:  defaultLinuxSans,
	Serif: defaultLinuxSerif,
	Mono:  defaultLinuxMono,
}

// defaultForMapped returns the last-resort hardcoded family for a mapped
// family, falling back to the sans-serif default.
//
// It matches on the mapped family rather than asking fontgeneric what the
// written name was, which looks like the same question and is not: a person
// who writes "DejaVu Serif" and has no DejaVu installed keeps the serif
// baseline here, where classifying the name again would hand them the sans
// one. Both answers are a hardcoded string for a font that is missing either
// way, so this keeps the one it has always given.
func defaultForMapped(mapped string) string {
	switch mapped {
	case defaultLinuxSerif:
		return defaultLinuxSerif
	case defaultLinuxMono:
		return defaultLinuxMono
	default:
		return defaultLinuxSans
	}
}
