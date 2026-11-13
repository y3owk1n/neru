package app_test

import (
	"context"
	"image"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/ports"
)

// vimKeymapHarness is vimHarness with [vim.keymaps.normal] set to normal.
func vimKeymapHarness(
	t *testing.T,
	normal map[string]string,
	words map[string]image.Rectangle,
	adjust ...func(*config.Config),
) *simHarness {
	t.Helper()

	cfg := simConfig()
	cfg.Hotkeys.Bindings[vimHotkey] = []string{domain.ModeNameVim}
	cfg.Vim.Keymaps.Normal = normal

	for _, apply := range adjust {
		apply(cfg)
	}

	sim := newSimHarness(t, cfg, nil)
	simWords(t, sim, words)

	return sim
}

// vimThreeWords are File and Edit on one line and Exit below them.
var vimThreeWords = map[string]image.Rectangle{
	vimWordFile: image.Rect(100, 100, 160, 130),
	vimWordEdit: image.Rect(200, 100, 260, 130),
	vimWordExit: image.Rect(100, 200, 160, 230),
}

// TestSimulation_VimKeymapsMoveVimModesCommandsToOtherKeys covers a mapping
// to one of vim mode's own commands. Space j starts the jump, and s, mapped
// to nothing, no longer does, so the j and the text after it reach the jump.
func TestSimulation_VimKeymapsMoveVimModesCommandsToOtherKeys(t *testing.T) {
	sim := vimKeymapHarness(t, map[string]string{
		"<Space>j": "<Plug>(jump)",
		"s":        "<Nop>",
	}, vimThreeWords)

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("s", "Space", "j", "e", "x", "i", "Return")
	vimFrameShows(sim, "cursor on the E of Exit", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(100, 200, 115, 230) && len(f.Labels) == 0
	})
}

// TestSimulation_VimKeymapsScrollAtTheEdgeForAKeyMappedToJ covers a layout
// that moves j, as Colemak users do. A key mapped to j scrolls the app past
// the last line read, as j itself does.
func TestSimulation_VimKeymapsScrollAtTheEdgeForAKeyMappedToJ(t *testing.T) {
	sim := vimKeymapHarness(t, map[string]string{"n": "j"},
		map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("n")
	sim.waitFor("the app scrolled down", func() bool {
		scrolls := sim.ax.recordedScrolls()

		return len(scrolls) == 1 && scrolls[0].Y < 0
	})
}

// TestSimulation_VimKeymapsRunHeldKeysOnceTheMappingTimesOut covers a key that
// starts a longer mapping. w waits for the rest of ww, and with nothing after
// it, runs as typed a second later, moving to the next word.
func TestSimulation_VimKeymapsRunHeldKeysOnceTheMappingTimesOut(t *testing.T) {
	sim := vimKeymapHarness(t, map[string]string{"ww": "$"}, vimThreeWords)

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("g", "g", "0", "w", "w")
	vimFrameShows(sim, "ww ran $, onto the t of Edit", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(245, 100, 260, 130)
	})

	// The second w ended the wait the first started, so nothing runs once
	// it would have timed out.
	time.Sleep(vimMapTimeoutForTest)

	if frame, _ := sim.overlay.lastVimFrame(); frame.Cursor != image.Rect(245, 100, 260, 130) {
		t.Fatalf("cursor moved to %v after ww, want it left on the t of Edit", frame.Cursor)
	}

	sim.press("0")
	vimFrameShows(sim, "cursor on the F of File", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(100, 100, 115, 130)
	})

	pressed := time.Now()

	sim.press("w")
	vimFrameShows(sim, "w ran as typed, onto the E of Edit", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(200, 100, 215, 130)
	})

	if waited := time.Since(pressed); waited < time.Second {
		t.Fatalf("w ran after %v, before the mapping timed out", waited)
	}
}

// vimMapTimeoutForTest is longer than vim mode waits for the rest of a
// mapping.
const vimMapTimeoutForTest = 1200 * time.Millisecond

// TestSimulation_VimKeymapsEscapeDropsHeldKeys covers Escape while a key
// waits for the rest of a mapping. It drops the key without running it, and
// the mode stays open until the next Escape.
func TestSimulation_VimKeymapsEscapeDropsHeldKeys(t *testing.T) {
	sim := vimKeymapHarness(t, map[string]string{"ww": "$"}, vimThreeWords)

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("g", "g", "0", "w", "Escape")
	vimFrameShows(sim, "cursor on the F of File, nothing held", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(100, 100, 115, 130) && f.Status.Text == ""
	})

	time.Sleep(vimMapTimeoutForTest)

	if frame, _ := sim.overlay.lastVimFrame(); frame.Cursor != image.Rect(100, 100, 115, 130) {
		t.Fatalf("the dropped w moved the cursor to %v", frame.Cursor)
	}

	sim.waitMode(domain.ModeVim)

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_VimKeymapsScrollSidewaysAfterAnEdgeScroll covers zl typed
// while the screen j scrolled at the edge is still being read. The z waits
// for its l, and the read landing in between must not feed j into it.
func TestSimulation_VimKeymapsScrollSidewaysAfterAnEdgeScroll(t *testing.T) {
	sim := vimKeymapHarness(t, nil,
		map[string]image.Rectangle{vimWordFile: image.Rect(100, 100, 160, 130)})
	gate := gateVimReads(sim)

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)
	vimFrameShows(sim, "the activation's accurate read landed", func(f ports.VimFrame) bool {
		return !strings.Contains(f.Status.Right, "reading")
	})

	gate.armed.Store(true)

	sim.press("j")
	sim.waitFor("the app scrolled down", func() bool { return len(sim.ax.recordedScrolls()) == 1 })

	time.Sleep(vimScrollSettleForTest)
	sim.press("z")
	vimFrameShows(sim, "z waiting while the screen is read", func(f ports.VimFrame) bool {
		return f.Status.Text == "z"
	})

	close(gate.release)
	vimFrameShows(sim, "the scrolled screen read, z still waiting", func(f ports.VimFrame) bool {
		return !strings.Contains(f.Status.Right, "reading") && f.Status.Text == "z"
	})

	sim.press("l")
	sim.waitFor("the app scrolled sideways", func() bool {
		scrolls := sim.ax.recordedScrolls()

		return len(scrolls) == 2 && scrolls[1].X != 0 && scrolls[1].Y == 0
	})
}

// TestSimulation_VimKeymapsStartAJumpWithTheKeysHeldAfterIt covers a jump that
// opens while the key typed after it waits for a longer mapping. s waits for
// sa, so the e after it runs the jump, and e itself waits for ex. That e was
// typed into the jump, so it begins the jump's text.
func TestSimulation_VimKeymapsStartAJumpWithTheKeysHeldAfterIt(t *testing.T) {
	sim := vimKeymapHarness(t, map[string]string{"sa": "x", "ex": "$"}, vimThreeWords)

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	sim.press("s", "e", "x", "i", "Return")
	vimFrameShows(sim, "cursor on the E of Exit", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(100, 200, 115, 230) && len(f.Labels) == 0
	})
}

// TestSimulation_VimKeymapsRunSeveralCommandsInOrder covers a mapping that
// runs two of vim mode's commands: s reads the screen again, then jumps.
func TestSimulation_VimKeymapsRunSeveralCommandsInOrder(t *testing.T) {
	sim := vimKeymapHarness(t, map[string]string{
		"s": "<Plug>(read-screen)<Plug>(jump)",
	}, vimThreeWords)

	var reads atomic.Int32

	read := sim.vision.ReadLinesFunc
	sim.vision.ReadLinesFunc = func(
		ctx context.Context, region image.Rectangle, level ports.TextLevel,
	) ([][]ports.TextPiece, error) {
		if level == ports.TextFast && !region.Empty() {
			reads.Add(1)
		}

		return read(ctx, region, level)
	}

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	before := reads.Load()

	sim.press("s")
	vimFrameShows(sim, "the jump waiting for text", func(f ports.VimFrame) bool {
		return f.Status.Text == "s" && f.Status.Typed
	})

	if reads.Load() != before+1 {
		t.Fatalf("s read the screen %d times, want once", reads.Load()-before)
	}

	sim.press("e", "x", "i", "Return")
	vimFrameShows(sim, "cursor on the E of Exit", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(100, 200, 115, 230)
	})
}

// TestSimulation_VimHotkeyOpensStraightIntoAJump covers a hotkey that opens
// vim mode and feeds it s, so the jump is waiting as the mode opens.
func TestSimulation_VimHotkeyOpensStraightIntoAJump(t *testing.T) {
	sim := vimKeymapHarness(t, nil, vimThreeWords, func(cfg *config.Config) {
		cfg.Hotkeys.Bindings[vimHotkey] = []string{domain.ModeNameVim, "action feed --mode s"}
	})

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	vimFrameShows(sim, "the jump waiting for text", func(f ports.VimFrame) bool {
		return f.Status.Text == "s" && f.Status.Typed
	})

	sim.press("e", "x", "i", "Return")
	vimFrameShows(sim, "cursor on the E of Exit", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(100, 200, 115, 230)
	})
}

// TestSimulation_VimKeymapsHoldBackTheCtrlChordsTheyMap covers a mapping typed
// with a Ctrl chord vim has no use for. The tap would pass it through to the
// app, so an open session holds it back for the mapping.
func TestSimulation_VimKeymapsHoldBackTheCtrlChordsTheyMap(t *testing.T) {
	sim := vimKeymapHarness(t, map[string]string{"<C-j>": "<Plug>(jump)"}, vimThreeWords,
		func(cfg *config.Config) { cfg.General.PassthroughUnboundedKeys = true })

	sim.pressHotkey(vimHotkey)
	sim.waitMode(domain.ModeVim)

	chord := config.CanonicalHotkeyForPlatform("Ctrl+J")

	sim.waitFor("Ctrl+J held back from the app", func() bool {
		_, blacklist := sim.tap.ModifierPassthrough()

		return slices.Contains(sim.tap.InterceptedModifierKeys(), chord) &&
			slices.Contains(blacklist, chord)
	})

	sim.press("Ctrl+j", "e", "x", "i", "Return")
	vimFrameShows(sim, "cursor on the E of Exit", func(f ports.VimFrame) bool {
		return f.Cursor == image.Rect(100, 200, 115, 230)
	})
}
