//go:build windows

package windows //nolint:testpackage // exercises unexported color-blend helpers directly

import "testing"

func TestArgbToGDIColorRef_opaqueUsesRGB(t *testing.T) {
	const argb = uint32(0xFF17327A)

	got := argbToGDIColorRef(argb, themeSurfaceLight)
	want := rgbToColorRef(0x17, 0x32, 0x7A)

	if got != want {
		t.Fatalf("got %#x want %#x", got, want)
	}
}

func TestArgbToGDIColorRef_blendsSemiTransparentBorder(t *testing.T) {
	// #99465FBC border over light surface #EEF2FF
	const argb = uint32(0x99465FBC)

	got := argbToGDIColorRef(argb, themeSurfaceLight)

	// Expected: alpha blend over EEF2FF
	alpha := uint16(0x99)
	inv := 255 - alpha
	wantR := uint8((uint16(0x46)*alpha + uint16(0xEE)*inv) / 255)
	wantG := uint8((uint16(0x5F)*alpha + uint16(0xF2)*inv) / 255)
	wantB := uint8((uint16(0xBC)*alpha + uint16(0xFF)*inv) / 255)
	want := rgbToColorRef(wantR, wantG, wantB)

	if got != want {
		t.Fatalf("got %#x want %#x", got, want)
	}
}

func TestArgbToGDIColorRef_avoidsColorKey(t *testing.T) {
	got := argbToGDIColorRef(0xFF010101, themeSurfaceLight)

	if got == overlayColorKey {
		t.Fatalf("color key collision was not avoided")
	}
}
