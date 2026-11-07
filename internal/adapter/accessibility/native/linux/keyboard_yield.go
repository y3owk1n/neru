//go:build linux

package linux

import (
	"strings"
	"sync"

	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
	linuxplatform "github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// The yielder is a process-global slot like the synthetic modifier sink, for
// the same reason: FeedKeyX11 is a package-level function. The live tap is
// wired at daemon startup and cleared at teardown
// (internal/app/wiring_platform_linux.go). A nil yielder means no tap is
// listening, as in the CLI and tests, and a key is sent as is.
var (
	keyboardYielderMu sync.RWMutex
	keyboardYielder   tap.KeyboardYielder
)

// SetKeyboardYielder wires the live event tap into X11 key feeding. Passing
// nil unwires it.
func SetKeyboardYielder(yielder tap.KeyboardYielder) {
	keyboardYielderMu.Lock()
	keyboardYielder = yielder
	keyboardYielderMu.Unlock()
}

// FeedKeyX11 types a key or chord such as "Ctrl+Shift+Space", in the form
// linuxplatform.FeedKey takes, into the focused window through XTest.
//
// The tap steps aside for the length of the key (tap.KeyboardYielder). XTest
// is used rather than uinput because it returns once the server has handled
// the key, so the grab cannot come back before the key has gone to the window. Modifiers go
// through the same hold as a click's, so a modifier the user holds is neither
// pressed twice nor released under them.
func FeedKeyX11(key string) error {
	names, keycode, err := linuxplatform.ParseFeedKey(key)
	if err != nil {
		return err
	}

	modifiers, err := action.ParseModifiers(strings.Join(names, ","))
	if err != nil {
		return err
	}

	send := func() error { return x11FeedKey(modifiers, keycode) }

	keyboardYielderMu.RLock()

	yielder := keyboardYielder

	keyboardYielderMu.RUnlock()

	if yielder == nil {
		return send()
	}

	return yielder.YieldKeyboard(send)
}
