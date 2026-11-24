//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: cairo
#include "../../platform/linux/badge_paint.h"
*/
import "C"

import (
	"unicode/utf8"
	"unsafe"
)

// cLook is look as the C badge painters take it. Text and family longer
// than the C buffers hold are cut at a rune boundary.
func (look badgeLook) cLook() C.NeruBadgeLook {
	var out C.NeruBadgeLook

	out.width = C.int(look.width)
	out.height = C.int(look.height)
	out.pad = C.int(look.pad)
	out.fill = C.uint(look.fill)
	out.stroke = C.uint(look.stroke)
	out.stroke_width = C.double(look.strokeWidth)
	out.text_color = C.uint(look.textColor)
	out.font_size = C.double(look.fontSize)

	if look.bold {
		out.bold = 1
	}

	copyCString(unsafe.Pointer(&out.text[0]), len(out.text), look.text)
	copyCString(unsafe.Pointer(&out.family[0]), len(out.family), look.family)

	return out
}

// copyCString writes text into the C char array at dst of capacity bytes,
// NUL-terminated and cut at a rune boundary when it does not fit.
func copyCString(dst unsafe.Pointer, capacity int, text string) {
	for len(text) > capacity-1 {
		_, size := utf8.DecodeLastRuneInString(text)
		text = text[:len(text)-size]
	}

	buf := unsafe.Slice((*byte)(dst), capacity)
	n := copy(buf, text)
	buf[n] = 0
}
