//go:build linux

package linux

import "github.com/y3owk1n/neru/internal/adapter/platform/fontgeneric"

const (
	// defaultLinuxSans is the baseline Linux sans-serif family, used when
	// fontconfig is absent or the family is missing.
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

// defaultForMapped returns the hardcoded last-resort family for a mapped
// family, defaulting to sans. It matches the mapped family rather than
// reclassifying the written name, so someone who writes "DejaVu Serif" without
// DejaVu installed keeps the serif baseline instead of getting sans.
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
