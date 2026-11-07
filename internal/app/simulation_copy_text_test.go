package app_test

// Journeys for copy_text: copying the selected hint's text to the clipboard,
// as a mode's action, as a select_with step and as an --on-exit step.

import (
	"context"
	"image"
	"slices"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
)

const (
	copyTextTitle = "Save"
	stepCopyText  = "action copy_text"
	stepIdle      = "idle"
	hintsCopyText = "hints --action copy_text"
)

// saveButton is the one-element fixture the copy journeys select.
func saveButton(t *testing.T) []*element.Element {
	t.Helper()

	return []*element.Element{simElement(t, "save", image.Rect(100, 100, 220, 140), copyTextTitle)}
}

// TestSimulation_CopyText_AsTheModeActionCopiesWithoutClicking covers the
// copy hotkey. Typing a label copies its hint's text and exits, and nothing is
// clicked.
func TestSimulation_CopyText_AsTheModeActionCopiesWithoutClicking(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{hintsCopyText}

	sim := newSimHarness(t, cfg, saveButton(t))

	sim.typeLabel(sim.openHints()[0])

	sim.waitFor("the text copied", func() bool { return len(sim.clipboard.Written()) == 1 })
	sim.waitMode(domain.ModeIdle)

	if got := sim.clipboard.Written(); got[0] != copyTextTitle {
		t.Errorf("clipboard = %q, want the hint's text %q", got[0], copyTextTitle)
	}

	if clicks := sim.ax.recordedClicks(); len(clicks) != 0 {
		t.Errorf("copy_text clicked %v, want no click", clicks)
	}
}

// TestSimulation_CopyText_RepeatCopiesEachSelection pins that a --repeat copy
// session reopens hints after each copy, as a click session does.
func TestSimulation_CopyText_RepeatCopiesEachSelection(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{"hints --action copy_text --repeat"}

	sim := newSimHarness(t, cfg, saveButton(t))

	labels := sim.openHints()
	drawsBefore := sim.overlay.hintDrawCount()

	sim.typeLabel(labels[0])
	sim.waitFor("the text copied", func() bool { return len(sim.clipboard.Written()) == 1 })
	sim.waitFor("hints reopened", func() bool { return sim.overlay.hintDrawCount() > drawsBefore })

	sim.typeLabel(sim.overlay.lastHintLabels()[0])
	sim.waitFor("the text copied again", func() bool { return len(sim.clipboard.Written()) == 2 })

	if sim.app.CurrentMode() != domain.ModeHints {
		t.Fatalf("mode = %v, want hints still open under --repeat", sim.app.CurrentMode())
	}
}

// TestSimulation_CopyText_AsAStepCopiesTheSelectedHint covers copy_text as a
// step, in a select_with entry and in a binding's --on-exit, where it copies
// the hint just selected.
func TestSimulation_CopyText_AsAStepCopiesTheSelectedHint(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(cfg *config.Config)
		typeKey func(sim *simHarness, label string)
	}{
		{
			"a select_with entry",
			func(cfg *config.Config) {
				cfg.Hotkeys.Bindings[hintsHotkey] = []string{stepHintsClick}
				cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
					selectWithCtrl: {stepCopyText, stepIdle},
				}
			},
			func(sim *simHarness, label string) { sim.typeLabelWith("Ctrl", label) },
		},
		{
			"an --on-exit step after a click",
			func(cfg *config.Config) {
				cfg.Hotkeys.Bindings[hintsHotkey] = []string{
					"hints --action=left_click --on-exit='" + stepCopyText + "'",
				}
			},
			func(sim *simHarness, label string) { sim.typeLabel(label) },
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := simConfig()
			testCase.mutate(cfg)

			sim := newSimHarness(t, cfg, saveButton(t))

			testCase.typeKey(sim, sim.openHints()[0])

			sim.waitFor("the text copied", func() bool {
				return slices.Equal(sim.clipboard.Written(), []string{copyTextTitle})
			})
			sim.waitMode(domain.ModeIdle)
		})
	}
}

// TestSimulation_CopyText_RefusesWhatItCannotCopy pins the two refusals. With
// no hint selected there is nothing to copy, and a hint with no text, such as
// one contour or segment found, leaves the clipboard alone.
func TestSimulation_CopyText_RefusesWhatItCannotCopy(t *testing.T) {
	runCopyText := func(sim *simHarness) ipc.Response {
		return sim.app.HandleCommand(context.Background(), ipc.Command{
			Action: domain.CommandAction,
			Args:   []string{"copy_text"},
		})
	}

	t.Run("no hint selected", func(t *testing.T) {
		sim := newSimHarness(t, simConfig(), saveButton(t))

		resp := runCopyText(sim)
		if resp.Success || resp.Code != ipc.CodeInvalidInput {
			t.Fatalf("copy_text with no selection = %+v, want ERR_INVALID_INPUT", resp)
		}
	})

	t.Run("a hint with no text", func(t *testing.T) {
		cfg := simConfig()
		cfg.Hotkeys.Bindings[hintsHotkey] = []string{hintsCopyText}

		blank := simElement(t, "shape", image.Rect(100, 100, 220, 140), "")
		sim := newSimHarness(t, cfg, []*element.Element{blank})

		sim.typeLabel(sim.openHints()[0])
		sim.waitMode(domain.ModeIdle)

		// Long enough for a copy that should not happen to land.
		time.Sleep(postSelectionQuiet)

		if got := sim.clipboard.Written(); len(got) != 0 {
			t.Fatalf("clipboard = %q, want nothing written for a hint with no text", got)
		}

		resp := runCopyText(sim)
		if resp.Success || resp.Code != ipc.CodeActionFailed {
			t.Fatalf("copy_text on a hint with no text = %+v, want ERR_ACTION_FAILED", resp)
		}
	})
}

// TestSimulation_CopyText_ConfirmingASearchCopiesTheMatch covers the hint
// search path. Confirming the search field in a copy_text session copies the
// match instead of clicking it.
func TestSimulation_CopyText_ConfirmingASearchCopiesTheMatch(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{"hints --action copy_text --search"}

	elements := threeButtons(t)
	sim := newSimHarness(t, cfg, elements)
	sim.textInput.Started = true

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.waitFor("search field opened", func() bool { return sim.textInput.StartCount() == 1 })

	callbacks := sim.textInput.Callbacks()
	callbacks.OnQueryChanged("cancel")
	sim.waitFor("hints narrowed to the query", func() bool {
		return len(sim.overlay.lastHintLabels()) == 1
	})

	callbacks.OnConfirm()

	sim.waitFor("the match copied", func() bool {
		return slices.Equal(sim.clipboard.Written(), []string{"Cancel"})
	})
	sim.waitMode(domain.ModeIdle)

	if clicks := sim.ax.recordedClicks(); len(clicks) != 0 {
		t.Errorf("confirming a copy_text search clicked %v, want no click", clicks)
	}
}

// TestSimulation_CopyText_ASlowClipboardNeverHoldsUpKeys pins that the
// clipboard is written off the mode handler's lock. A copier can take up to its
// deadline, and a key pressed meanwhile must still be handled.
func TestSimulation_CopyText_ASlowClipboardNeverHoldsUpKeys(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{hintsCopyText}

	sim := newSimHarness(t, cfg, saveButton(t))

	writing := make(chan struct{})
	release := make(chan struct{})
	sim.clipboard.WriteTextFunc = func(context.Context, string) error {
		close(writing)
		<-release

		return nil
	}

	sim.typeLabel(sim.openHints()[0])

	select {
	case <-writing:
	case <-time.After(simWaitHeadroom):
		t.Fatal("the clipboard write never started")
	}

	pressed := make(chan struct{})

	go func() {
		sim.press("Escape")
		close(pressed)
	}()

	select {
	case <-pressed:
	case <-time.After(simWaitHeadroom):
		close(release)
		t.Fatal("a key pressed during the clipboard write waited for it")
	}

	close(release)

	sim.waitMode(domain.ModeIdle)
	sim.waitFor("the text copied", func() bool { return len(sim.clipboard.Written()) == 1 })
}
