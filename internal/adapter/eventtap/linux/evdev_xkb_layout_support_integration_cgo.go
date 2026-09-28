// Test support for the reference-layout integration test, built only under the
// integration tag. It builds a capture whose xkb state comes from real layouts
// in the system's XKB data, with no compositor. Nothing here reaches a shipped
// binary.

//go:build integration && linux && cgo

package linux

/*
#cgo pkg-config: xkbcommon
#include <stdlib.h>
#include <xkbcommon/xkbcommon.h>
#include "../../platform/linux/wayland_keymap.h"

// neru_test_keymap_text compiles layouts by name the way a compositor does and
// serializes them the way wl_keyboard.keymap sends them. The caller frees it.
static char *neru_test_keymap_text(const char *layout, const char *variant, const char *options) {
	struct xkb_context *ctx = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
	if (!ctx)
		return NULL;

	struct xkb_rule_names names = {.layout = layout, .variant = variant, .options = options};
	struct xkb_keymap *keymap = xkb_keymap_new_from_names(ctx, &names, XKB_KEYMAP_COMPILE_NO_FLAGS);
	xkb_context_unref(ctx);
	if (!keymap)
		return NULL;

	char *text = xkb_keymap_get_as_string(keymap, XKB_KEYMAP_FORMAT_TEXT_V1);
	xkb_keymap_unref(keymap);
	return text;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

var errKeymapRefused = errors.New("xkbcommon could not compile the layouts")

// newTestXkbCapture returns a capture holding the xkb state a compositor would
// send for these layouts, variants and options.
func newTestXkbCapture(layout, variant, options string) (*waylandEvdevCapture, error) {
	cLayout := C.CString(layout)
	defer C.free(unsafe.Pointer(cLayout))

	cVariant := C.CString(variant)
	defer C.free(unsafe.Pointer(cVariant))

	cOptions := C.CString(options)
	defer C.free(unsafe.Pointer(cOptions))

	text := C.neru_test_keymap_text(cLayout, cVariant, cOptions)
	if text == nil {
		return nil, errKeymapRefused
	}
	defer C.free(unsafe.Pointer(text))

	state := C.neru_xkb_state_create_from_keymap(text)
	if state == nil {
		return nil, errKeymapRefused
	}

	return &waylandEvdevCapture{xkbState: unsafe.Pointer(state)}, nil
}

func (capture *waylandEvdevCapture) destroyTestXkb() {
	C.neru_xkb_state_destroy((*C.neru_xkb_state)(capture.xkbState))
	capture.xkbState = nil
}
