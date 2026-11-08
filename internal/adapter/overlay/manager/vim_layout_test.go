package manager_test

import (
	"image"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
)

// measureEightPerRune stands in for a font eight points wide per character.
func measureEightPerRune(text string, _ float64, _ bool) int { return 8 * len([]rune(text)) }

func TestLayoutVim_PutsTheStatusBarAlongTheRegionsBottomEdge(t *testing.T) {
	ops := manager.LayoutVim(manager.VimScene{
		Region: image.Rect(100, 100, 900, 700),
		Status: "NORMAL",
	}, manager.VimStyle{FontSize: 10}, measureEightPerRune)

	var bar, label *manager.VimOp

	for index := range ops {
		switch ops[index].Kind {
		case manager.VimOpRect:
			bar = &ops[index]
		case manager.VimOpText:
			label = &ops[index]
		case manager.VimOpRoundedRect:
		}
	}

	if bar == nil || bar.Bounds.Max.Y != 700 || bar.Bounds.Min.X != 100 || bar.Bounds.Max.X != 900 {
		t.Fatalf("status bar = %v, want the region's width along its bottom edge", bar)
	}

	if label == nil || label.Text != "NORMAL" || label.Bounds.Dx() != 48 ||
		!label.Bounds.In(bar.Bounds) {
		t.Fatalf("status text = %v, want NORMAL, 48 wide, inside the bar", label)
	}
}

func TestLayoutVim_PutsTheLensAboveTheCursorOrBelowWithNoRoom(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		cursor image.Rectangle
		above  bool
	}{
		{"room above", image.Rect(200, 300, 208, 316), true},
		{"top of the screen", image.Rect(200, 2, 208, 18), false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ops := manager.LayoutVim(manager.VimScene{
				Cursor: scenario.cursor,
				Lens:   [3]string{"wo", "r", "d"},
			}, manager.VimStyle{FontSize: 10}, measureEightPerRune)

			var lens image.Rectangle
			for _, op := range ops {
				if op.Kind == manager.VimOpRoundedRect {
					lens = op.Bounds
				}
			}

			if scenario.above && lens.Max.Y > scenario.cursor.Min.Y ||
				!scenario.above && lens.Min.Y < scenario.cursor.Max.Y {
				t.Fatalf(
					"lens = %v for cursor %v, want it above: %v",
					lens,
					scenario.cursor,
					scenario.above,
				)
			}

			if lens.Dx() != 4*8+2*4 {
				t.Fatalf("lens is %d wide, want the word's 32 plus padding", lens.Dx())
			}
		})
	}
}

func TestLayoutVim_SplitsTheNoticeIntoATitleAndADetail(t *testing.T) {
	ops := manager.LayoutVim(manager.VimScene{
		Region: image.Rect(0, 0, 1000, 800),
		Notice: "Preparing\nFirst time only",
	}, manager.VimStyle{FontSize: 10}, measureEightPerRune)

	var texts []manager.VimOp
	for _, op := range ops {
		if op.Kind == manager.VimOpText {
			texts = append(texts, op)
		}
	}

	if len(texts) != 2 || texts[0].Text != "Preparing" || !texts[0].Bold ||
		texts[1].Text != "First time only" {
		t.Fatalf("notice texts = %v, want a bold title then the detail", texts)
	}

	if texts[1].Bounds.Min.Y < texts[0].Bounds.Max.Y {
		t.Fatalf("detail at %v overlaps the title at %v", texts[1].Bounds, texts[0].Bounds)
	}
}

// TestLayoutVim_TurnsTheNoticeSpinnerWithItsStep pins that the waiting notice
// moves. A backend redraws it with the next step on a timer, so what the
// spinner draws has to change from one step to the next and come back after a
// full turn.
func TestLayoutVim_TurnsTheNoticeSpinnerWithItsStep(t *testing.T) {
	const maxTurn = 64

	spinner := func(step int) []image.Rectangle {
		ops := manager.LayoutVim(manager.VimScene{
			Region: image.Rect(0, 0, 1000, 800),
			Notice: "Reading the screen",
			Spin:   step,
		}, manager.VimStyle{FontSize: 10, CursorColor: "#336699"}, measureEightPerRune)

		var dots []image.Rectangle

		for _, drawn := range ops {
			if drawn.Kind == manager.VimOpRoundedRect &&
				drawn.Bounds.Dx() == drawn.Bounds.Dy() &&
				drawn.Radius*2 == float64(drawn.Bounds.Dx()) {
				dots = append(dots, drawn.Bounds)
			}
		}

		return dots
	}

	first := spinner(0)
	if len(first) < 2 {
		t.Fatalf("the notice drew %d spinner dots, want an arc of them", len(first))
	}

	if slices.Equal(spinner(1), first) {
		t.Fatalf("the spinner drew the same dots at step 0 and step 1: %v", first)
	}

	for step := 2; step <= maxTurn; step++ {
		if slices.Equal(spinner(step), first) {
			return
		}
	}

	t.Fatalf("the spinner never came back to its first step within %d steps", maxTurn)
}
