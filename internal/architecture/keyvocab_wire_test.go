package architecture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

// nativeKeyEventEmitters lists the native sources that format the synthetic
// "__keyup_"/"__modifier_" wire events with printf instead of going through
// internal/domain/keyvocab (C and Objective-C cannot import it). Each entry
// names the prefixes that file emits.
var nativeKeyEventEmitters = map[string][]string{
	"internal/adapter/platform/linux/overlay_wayland.c": {
		keyvocab.KeyUpPrefix,
		keyvocab.ModifierTogglePrefix,
	},
	"internal/adapter/platform/darwin/eventtap_darwin.m": {
		keyvocab.KeyUpPrefix,
		keyvocab.ModifierTogglePrefix,
	},
}

// TestNativeKeyEventEmittersMatchKeyvocab pins the two native emitters to the
// Go wire vocabulary. The synthetic key-up and modifier-toggle events cross a
// language boundary as bare printf format strings; if the Go prefixes in
// internal/domain/keyvocab ever change, this test fails on each native file
// that still emits the old spelling, instead of the protocol silently
// splitting between the taps and the mode handler.
//
// If this test fails because an emitter file was renamed or stopped emitting
// events, update nativeKeyEventEmitters to match reality.
func TestNativeKeyEventEmittersMatchKeyvocab(t *testing.T) {
	t.Parallel()

	root := findRepoRoot(t)

	for relPath, prefixes := range nativeKeyEventEmitters {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath)))
		if err != nil {
			t.Errorf(
				"%s: cannot read native key-event emitter (renamed?): %v — update nativeKeyEventEmitters",
				relPath,
				err,
			)

			continue
		}

		source := string(content)
		for _, prefix := range prefixes {
			if !strings.Contains(source, prefix) {
				t.Errorf(
					"%s: does not contain wire prefix %q from internal/domain/keyvocab; "+
						"the native emitter and the Go vocabulary must use the same spelling",
					relPath, prefix,
				)
			}
		}
	}
}
