//go:build linux

package linux

import "strings"

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

// mapGenericAlias translates fontconfig-style generic names (and empty
// input) to a known-good Linux family. Case- and whitespace-insensitive.
// Non-generic names are returned unchanged (trimmed) so the CGO path
// can ask fontconfig to verify them.
func mapGenericAlias(family string) string {
	switch strings.ToLower(strings.TrimSpace(family)) {
	case "", "sans", "sans-serif", "sansserif":
		return defaultLinuxSans
	case "serif":
		return defaultLinuxSerif
	case "mono", "monospace":
		return defaultLinuxMono
	default:
		return strings.TrimSpace(family)
	}
}

// defaultForMapped returns the last-resort hardcoded family for a mapped
// generic, falling back to the sans-serif default.
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
