package app_test

// Journeys for copy_text's second look at a hint found by reading the screen:
// re-read accurately while hints is open, the pass's text otherwise.

import (
	"context"
	"image"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
)

const (
	// passText is what a fast recognition pass misread a hint as, and
	// accurateText what a careful read of the same region returns.
	passText     = "denote5 breaking CLI chan9es."
	accurateText = "denotes breaking CLI changes."

	stepLeftClick = "action left_click"
)

// screenHintBounds is where the screen-found fixture hint sits.
var screenHintBounds = image.Rect(100, 100, 300, 130)

// textReads records the regions ReadText was asked for.
type textReads struct {
	mu    sync.Mutex
	rects []image.Rectangle
}

func (r *textReads) record(rect image.Rectangle) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.rects = append(r.rects, rect)
}

func (r *textReads) all() []image.Rectangle {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.rects)
}

// screenHintSim starts a harness whose hints come from the vision strategy,
// with one hint the pass read as passText, and binding as the hints hotkey.
// read answers ReadText, and every region it is asked for is recorded.
func screenHintSim(
	t *testing.T,
	binding string,
	read func(context.Context, image.Rectangle) (string, error),
) (*simHarness, *textReads) {
	t.Helper()

	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{binding}

	return screenHintSimWith(t, cfg, read)
}

// screenHintSimWith is screenHintSim for a configuration the caller built.
func screenHintSimWith(
	t *testing.T,
	cfg *config.Config,
	read func(context.Context, image.Rectangle) (string, error),
) (*simHarness, *textReads) {
	t.Helper()

	cfg.Hints.Strategy = domain.StrategyVision

	sim := newSimHarness(t, cfg, nil)
	reads := &textReads{}

	// A capture strategy scans the focused window, so the desktop needs one.
	sim.desktop.focusWindow(image.Rect(50, 50, 900, 700))

	// Hints mode keeps a vision element only when its role is one of this
	// platform's native clickable roles, so the fixture hint uses the native
	// button role rather than macOS's AXButton.
	vocabulary, _ := element.CurrentVocabulary()
	buttons, _ := element.LookupSemantic(element.SemanticButton)
	buttonRole := element.Role(buttons.Native(vocabulary)[0])

	sim.vision.DetectElementsFunc = func(
		context.Context,
		image.Rectangle,
		config.HintsVisionConfig,
		bool,
	) ([]*element.Element, error) {
		hint, err := element.NewElement(
			"vision-hint",
			screenHintBounds,
			buttonRole,
			element.WithSource(element.SourceVision),
			element.WithClickable(true),
			element.WithTitle(passText),
		)

		return []*element.Element{hint}, err
	}
	sim.vision.ReadTextFunc = func(ctx context.Context, rect image.Rectangle) (string, error) {
		reads.record(rect)

		return read(ctx, rect)
	}

	return sim, reads
}

func readAccurately(context.Context, image.Rectangle) (string, error) {
	return accurateText, nil
}

// TestSimulation_CopyText_RereadsAScreenHintWhileHintsIsOpen covers the case
// the second look exists for. The pass misread the hint, and the copy takes the
// careful read of the hint's own region instead.
func TestSimulation_CopyText_RereadsAScreenHintWhileHintsIsOpen(t *testing.T) {
	sim, reads := screenHintSim(t, hintsCopyText, readAccurately)

	sim.typeLabel(sim.openHints()[0])

	sim.waitFor("the text copied", func() bool { return len(sim.clipboard.Written()) == 1 })

	if got := sim.clipboard.Written()[0]; got != accurateText {
		t.Errorf("clipboard = %q, want the careful read %q", got, accurateText)
	}

	if got := reads.all(); len(got) != 1 || got[0] != screenHintBounds {
		t.Errorf("ReadText regions = %v, want one read of the hint's box %v", got, screenHintBounds)
	}
}

// TestSimulation_CopyText_FallsBackToThePassText pins that a second look that
// cannot help never costs the copy. The pass's text is copied when the
// platform has no accurate reader, the read fails, finds nothing, or runs out
// of time.
func TestSimulation_CopyText_FallsBackToThePassText(t *testing.T) {
	tests := []struct {
		name string
		read func(context.Context, image.Rectangle) (string, error)
	}{
		{"no accurate reader", func(context.Context, image.Rectangle) (string, error) {
			return "", derrors.New(derrors.CodeNotSupported, "not supported")
		}},
		{"a failed read", func(context.Context, image.Rectangle) (string, error) {
			return "", derrors.New(derrors.CodeActionFailed, "capture failed")
		}},
		{"a read that finds nothing", func(context.Context, image.Rectangle) (string, error) {
			return " ", nil
		}},
		{"a read past its deadline", func(ctx context.Context, _ image.Rectangle) (string, error) {
			<-ctx.Done()

			return "", ctx.Err()
		}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			sim, _ := screenHintSim(t, hintsCopyText, testCase.read)

			sim.typeLabel(sim.openHints()[0])

			sim.waitFor("the text copied", func() bool { return len(sim.clipboard.Written()) == 1 })

			if got := sim.clipboard.Written()[0]; got != passText {
				t.Errorf("clipboard = %q, want the pass's text %q", got, passText)
			}
		})
	}
}

// TestSimulation_CopyText_AfterHintsClosesCopiesThePassText pins the line the
// second look stops at. An --on-exit copy runs after the click and after hints
// closed, when the region may show something else, so it is not read again.
func TestSimulation_CopyText_AfterHintsClosesCopiesThePassText(t *testing.T) {
	sim, reads := screenHintSim(
		t,
		"hints --action=left_click --on-exit='"+stepCopyText+"'",
		readAccurately,
	)

	sim.typeLabel(sim.openHints()[0])

	sim.waitFor("the text copied", func() bool { return len(sim.clipboard.Written()) == 1 })

	if got := sim.clipboard.Written()[0]; got != passText {
		t.Errorf("clipboard = %q, want the pass's text %q", got, passText)
	}

	if got := reads.all(); len(got) != 0 {
		t.Errorf("ReadText regions = %v, want no read once hints closed", got)
	}
}

// TestSimulation_CopyText_AnAccessibilityHintIsNotReread pins that only a hint
// found on screen gets the second look. The accessibility tree's text is
// already exact.
func TestSimulation_CopyText_AnAccessibilityHintIsNotReread(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{hintsCopyText}

	sim := newSimHarness(t, cfg, saveButton(t))
	reads := &textReads{}
	sim.vision.ReadTextFunc = func(_ context.Context, rect image.Rectangle) (string, error) {
		reads.record(rect)

		return accurateText, nil
	}

	sim.typeLabel(sim.openHints()[0])

	sim.waitFor("the text copied", func() bool {
		return slices.Equal(sim.clipboard.Written(), []string{copyTextTitle})
	})

	if got := reads.all(); len(got) != 0 {
		t.Errorf("ReadText regions = %v, want no read for an accessibility hint", got)
	}
}

// TestSimulation_CopyText_ASlowRereadNeverHoldsUpKeys pins that the second
// look runs with the mode handler's lock released. A capture and a recognition
// can take until their deadline, and a key pressed meanwhile must still be
// handled.
func TestSimulation_CopyText_ASlowRereadNeverHoldsUpKeys(t *testing.T) {
	reading := make(chan struct{})
	release := make(chan struct{})

	sim, _ := screenHintSim(
		t,
		hintsCopyText,
		func(context.Context, image.Rectangle) (string, error) {
			close(reading)
			<-release

			return accurateText, nil
		},
	)

	sim.typeLabel(sim.openHints()[0])

	select {
	case <-reading:
	case <-time.After(simWaitHeadroom):
		t.Fatal("the second look never started")
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
		t.Fatal("a key pressed during the second look waited for it")
	}

	close(release)

	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_CopyText_ASelectWithEntryRereadsOnlyBeforeOtherSteps pins
// that a select_with entry reads the hint again only when no other step ran
// before the copy. A click before it may have changed what is under the hint
// while hints stays open, so the pass's text is copied instead.
func TestSimulation_CopyText_ASelectWithEntryRereadsOnlyBeforeOtherSteps(t *testing.T) {
	tests := []struct {
		name      string
		steps     []string
		wantText  string
		wantReads int
	}{
		{"copy first", []string{stepCopyText, stepIdle}, accurateText, 1},
		{
			"a click before the copy",
			[]string{stepLeftClick, stepCopyText, stepIdle},
			passText,
			0,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := simConfig()
			cfg.Hotkeys.Bindings[hintsHotkey] = []string{stepHintsClick}
			cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
				selectWithCtrl: testCase.steps,
			}

			sim, reads := screenHintSimWith(t, cfg, readAccurately)

			sim.typeLabelWith("Ctrl", sim.openHints()[0])

			sim.waitFor("the text copied", func() bool { return len(sim.clipboard.Written()) == 1 })

			if got := sim.clipboard.Written()[0]; got != testCase.wantText {
				t.Errorf("clipboard = %q, want %q", got, testCase.wantText)
			}

			if got := reads.all(); len(got) != testCase.wantReads {
				t.Errorf("ReadText regions = %v, want %d read(s)", got, testCase.wantReads)
			}
		})
	}
}
