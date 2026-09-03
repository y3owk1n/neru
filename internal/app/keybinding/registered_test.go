package keybinding_test

// What the event taps are told about global hotkeys is the set registration
// actually took, not the set the configuration asked for. Two of the three taps
// hand a chord in that set straight back to the backend that owns it rather than
// dispatching it, so a chord the backend refused must not be in it: nobody would
// run it at all.

import (
	"errors"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/app/keybinding"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

const (
	takenChord   = "Ctrl+Alt+G"
	refusedChord = "Ctrl+Alt+H"
)

// errChordTaken is the shape of a backend refusing a chord another process owns.
var errChordTaken = errors.New("hotkey already registered by another process")

func TestBinder_Register_PublishesOnlyTheChordsTheBackendTook(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hotkeys.Bindings = map[string][]string{
		takenChord:   {config.ModeNameGrid},
		refusedChord: {config.ModeNameScroll},
	}

	manager := &portmocks.MockHotkeyPort{}
	manager.RegisterFunc = func(keyString string, _ ports.HotkeyCallback) (ports.HotkeyID, error) {
		// The shape of a chord another process already owns.
		if keyString == config.CanonicalHotkeyForPlatform(refusedChord) {
			return 0, errChordTaken
		}

		return 1, nil
	}

	var published []string

	appState := state.NewAppState()
	appState.SetEnabled(true)

	binder := keybinding.New(keybinding.Deps{
		Manager:                  manager,
		State:                    appState,
		Config:                   func() *config.Config { return cfg },
		RunSequence:              func(string, []string) {},
		PublishRegisteredHotkeys: func(keys []string) { published = keys },
	})

	binder.Register("")

	want := config.CanonicalHotkeyForPlatform(takenChord)
	if !slices.Contains(published, want) {
		t.Errorf("published %v, want %q among them", published, want)
	}

	if refused := config.CanonicalHotkeyForPlatform(
		refusedChord,
	); slices.Contains(
		published,
		refused,
	) {
		t.Errorf(
			"published %v, want %q absent: the backend refused it, so handing it back "+
				"to the backend would leave nothing running it",
			published, refused,
		)
	}
}

// A binder wired without the hook still registers. It is optional because the
// hotkey table is built before the event tap exists on some startup paths.
func TestBinder_Register_WithoutThePublishHook(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hotkeys.Bindings = map[string][]string{takenChord: {config.ModeNameGrid}}

	manager := &portmocks.MockHotkeyPort{}

	appState := state.NewAppState()
	appState.SetEnabled(true)

	binder := keybinding.New(keybinding.Deps{
		Manager:     manager,
		State:       appState,
		Config:      func() *config.Config { return cfg },
		RunSequence: func(string, []string) {},
	})

	binder.Register("")

	if got := manager.RegisteredKeys(); len(got) != 1 {
		t.Errorf("registered %v, want the one configured chord", got)
	}
}
