//go:build windows

// internal/core/infra/platform/windows/overlay_color.go
// Converts Neru AARRGGBB colors to opaque Win32 COLORREF values for GDI painting.
// Does not implement drawing; overlay.go uses these helpers during WM_PAINT.

package windows

// overlayColorKey is the Win32 COLORREF made transparent via SetLayeredWindowAttributes.
// Use a rare RGB value so grid theme colors are not punched through as holes.
const overlayColorKey = 0x00010101 // RGB(1, 1, 1)

func rgbToColorRef(red, green, blue uint8) uint32 {
	return uint32(blue) | (uint32(green) << 8) | (uint32(red) << 16)
}

// argbToGDIColorRef converts AARRGGBB to an opaque COLORREF for GDI.
// Semi-transparent theme colors are alpha-blended over blendRGB, matching how
// cairo composites grid strokes and labels on Linux/macOS.
func argbToGDIColorRef(argb uint32, blendRGB uint32) uint32 {
	alpha := uint8(argb >> 24)
	red := uint8(argb >> 16)
	green := uint8(argb >> 8)
	blue := uint8(argb)

	if alpha < 255 {
		blendR := uint8(blendRGB >> 16)
		blendG := uint8(blendRGB >> 8)
		blendB := uint8(blendRGB)

		inv := 255 - uint16(alpha)
		red = uint8((uint16(red)*uint16(alpha) + uint16(blendR)*inv) / 255)
		green = uint8((uint16(green)*uint16(alpha) + uint16(blendG)*inv) / 255)
		blue = uint8((uint16(blue)*uint16(alpha) + uint16(blendB)*inv) / 255)
	}

	return avoidColorKey(rgbToColorRef(red, green, blue))
}

func avoidColorKey(ref uint32) uint32 {
	if ref == overlayColorKey {
		return ref + 1
	}

	return ref
}
