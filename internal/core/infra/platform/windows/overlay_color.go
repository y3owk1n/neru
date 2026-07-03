//go:build windows

// internal/core/infra/platform/windows/overlay_color.go
// Converts Neru AARRGGBB colors to opaque Win32 COLORREF values for GDI painting.
// Does not implement drawing; overlay.go uses these helpers during WM_PAINT.

package windows

// overlayColorKey is the Win32 COLORREF made transparent via SetLayeredWindowAttributes.
// Use a rare RGB value so grid theme colors are not punched through as holes.
const overlayColorKey = 0x00010101 // RGB(1, 1, 1)

// Bit offsets for the channels of an AARRGGBB / 0xRRGGBB packed color, and the
// maximum value of a single 8-bit channel.
const (
	greenShift = 8
	redShift   = 16
	alphaShift = 24
	maxChannel = 255
)

// rgbToColorRef converts individual R/G/B channels into a Win32 COLORREF.
// COLORREF format is 0x00BBGGRR: B in the highest byte, R in the lowest byte.
func rgbToColorRef(red, green, blue uint8) uint32 {
	return uint32(red) | (uint32(green) << greenShift) | (uint32(blue) << redShift)
}

// argbToGDIColorRef converts AARRGGBB to an opaque COLORREF for GDI.
// Semi-transparent theme colors are alpha-blended over blendRGB, matching how
// cairo composites grid strokes and labels on Linux/macOS.
func argbToGDIColorRef(argb uint32, blendRGB uint32) uint32 {
	alpha := uint8(argb >> alphaShift)
	red := uint8(argb >> redShift)
	green := uint8(argb >> greenShift)
	blue := uint8(argb)

	if alpha < maxChannel {
		blendR := uint8(blendRGB >> redShift)
		blendG := uint8(blendRGB >> greenShift)
		blendB := uint8(blendRGB)

		inv := maxChannel - uint16(alpha)
		red = uint8((uint16(red)*uint16(alpha) + uint16(blendR)*inv) / maxChannel)
		green = uint8((uint16(green)*uint16(alpha) + uint16(blendG)*inv) / maxChannel)
		blue = uint8((uint16(blue)*uint16(alpha) + uint16(blendB)*inv) / maxChannel)
	}

	return avoidColorKey(rgbToColorRef(red, green, blue))
}

func avoidColorKey(ref uint32) uint32 {
	if ref == overlayColorKey {
		return ref + 1
	}

	return ref
}
