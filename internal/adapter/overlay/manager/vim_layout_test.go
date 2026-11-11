package manager_test

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
)

// measureEightPerRune stands in for a font eight points wide per character.
func measureEightPerRune(text, _ string, _ float64, _ bool) int { return 8 * len([]rune(text)) }

func TestLayoutVim_PutsTheStatusBarAlongTheRegionsBottomEdge(t *testing.T) {
	ops := manager.LayoutVim(manager.VimScene{
		Region: image.Rect(100, 100, 900, 700),
		Mode:   "NORMAL",
	}, manager.VimStyle{FontSize: 10}, measureEightPerRune)

	if len(ops) == 0 || ops[0].Kind != manager.VimOpRect {
		t.Fatalf("ops = %v, want the bar first", ops)
	}

	bar := ops[0].Bounds
	if bar.Max.Y != 700 || bar.Min.X != 100 || bar.Max.X != 900 {
		t.Fatalf("status bar = %v, want the region's width along its bottom edge", bar)
	}

	for _, op := range ops {
		if !op.Bounds.In(bar) {
			t.Fatalf("%v at %v falls outside the bar %v", op.Kind, op.Bounds, bar)
		}
	}
}

// TestLayoutVim_ShowsTheModeAsAChipThatSaysWhichKind pins the chip's look
// per kind of mode. Normal and visual each fill it in their own color with
// OnModeColor text, and a mode waiting on input outlines it.
func TestLayoutVim_ShowsTheModeAsAChipThatSaysWhichKind(t *testing.T) {
	style := manager.VimStyle{
		FontSize:        10,
		StatusTextColor: "#111111",
		ModeColor:       "#2222AA",
		VisualModeColor: "#AA22AA",
		OnModeColor:     "#FFFFFF",
		Badge:           manager.VimBadge{Border: "#3333CC"},
	}

	for _, scenario := range []struct {
		mode               string
		kind               manager.VimModeKind
		fill, border, text uint32
	}{
		{"NORMAL", manager.VimModeNormal, 0xFF2222AA, 0, 0xFFFFFFFF},
		{"VISUAL", manager.VimModeVisual, 0xFFAA22AA, 0, 0xFFFFFFFF},
		{"COMMAND", manager.VimModeOther, 0, 0xFF3333CC, 0xFF111111},
	} {
		t.Run(scenario.mode, func(t *testing.T) {
			ops := manager.LayoutVim(manager.VimScene{
				Region:   image.Rect(0, 0, 800, 600),
				Mode:     scenario.mode,
				ModeKind: scenario.kind,
			}, style, measureEightPerRune)

			chip := slices.IndexFunc(ops, func(op manager.VimOp) bool {
				return op.Kind == manager.VimOpRoundedRect
			})
			if chip < 0 || chip+1 >= len(ops) || ops[chip+1].Text != scenario.mode {
				t.Fatalf("ops = %v, want a chip lettered %s", ops, scenario.mode)
			}

			if ops[chip].Fill != scenario.fill || ops[chip].Border != scenario.border ||
				ops[chip+1].Fill != scenario.text {
				t.Fatalf(
					"chip fill %#x border %#x text %#x, want %#x %#x %#x",
					ops[chip].Fill, ops[chip].Border, ops[chip+1].Fill,
					scenario.fill, scenario.border, scenario.text,
				)
			}
		})
	}
}

// TestLayoutVim_CutsTheStatusShortOfTheRightStatus pins that a long status
// never runs into the match count at the bar's right end. A message is cut at
// its end, and a command line being typed at its start, so what is typed next
// stays in view.
func TestLayoutVim_CutsTheStatusShortOfTheRightStatus(t *testing.T) {
	long := strings.Repeat("substitute ", 10) + "end"

	for _, scenario := range []struct {
		name  string
		typed bool
		keeps func(string) bool
	}{
		{"message keeps its start", false, func(shown string) bool {
			return strings.HasPrefix(shown, "substitute") && strings.HasSuffix(shown, "…")
		}},
		{"typed line keeps its end", true, func(shown string) bool {
			return strings.HasPrefix(shown, "…") && strings.HasSuffix(shown, "end")
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ops := manager.LayoutVim(manager.VimScene{
				Region:      image.Rect(0, 0, 400, 300),
				Mode:        "COMMAND",
				ModeKind:    manager.VimModeOther,
				Status:      long,
				StatusRight: "[1/3]",
				StatusTyped: scenario.typed,
			}, manager.VimStyle{FontSize: 10}, measureEightPerRune)

			var status, right *manager.VimOp

			for index := range ops {
				switch {
				case ops[index].Text == "[1/3]":
					right = &ops[index]
				case strings.Contains(ops[index].Text, "substitute"):
					status = &ops[index]
				}
			}

			if right == nil || right.Bounds.Max.X > 400 {
				t.Fatalf("right status = %v, want it inside the bar's right end", right)
			}

			if status == nil || !scenario.keeps(status.Text) ||
				status.Bounds.Max.X >= right.Bounds.Min.X {
				t.Fatalf("status = %v, want it cut before %v", status, right.Bounds)
			}
		})
	}
}

// TestLayoutVim_DrawsJumpLabelsAsHintBadges pins that an s jump's labels look
// like hint labels, in the hint badge's colors, font and padding, centered on
// the character each names, rather than in colors of their own.
func TestLayoutVim_DrawsJumpLabelsAsHintBadges(t *testing.T) {
	char := image.Rect(300, 200, 308, 216)
	ops := manager.LayoutVim(manager.VimScene{
		Labels: []manager.VimLabel{{Text: "a", Bounds: char}},
	}, manager.VimStyle{
		FontSize:        13,
		MatchColor:      "#0B2377",
		StatusTextColor: "#17327A",
		Badge: manager.VimBadge{
			FontSize: 10, FontFamily: "Hint Face", Background: "#EEF2FF",
			Text: "#17327A", Border: "#465FBC", BorderWidth: 1,
			PaddingX: 6, PaddingY: 4, BorderRadius: -1,
		},
	}, measureEightPerRune)

	if len(ops) != 2 || ops[0].Kind != manager.VimOpRoundedRect ||
		ops[1].Kind != manager.VimOpText {
		t.Fatalf("ops = %v, want one badge and its text", ops)
	}

	box, text := ops[0], ops[1]
	if box.Fill != 0xFFEEF2FF || box.Border != 0xFF465FBC || box.LineWidth != 1 {
		t.Fatalf(
			"badge fill %#x border %#x width %v, want the hint badge's",
			box.Fill,
			box.Border,
			box.LineWidth,
		)
	}

	if text.Fill != 0xFF17327A || text.Family != "Hint Face" || text.FontSize != 10 || !text.Bold {
		t.Fatalf("label text = %+v, want the hint badge's bold face and color", text)
	}

	if box.Bounds.Dx() != 8+2*6 {
		t.Fatalf("badge is %d wide, want the label's 8 plus the hint padding", box.Bounds.Dx())
	}

	center := char.Min.Add(char.Size().Div(2))
	if got := box.Bounds.Min.Add(
		box.Bounds.Size().Div(2),
	); got.Sub(center).X > 1 ||
		got.Sub(center).Y > 1 {
		t.Fatalf("badge centered at %v, want it on the character's center %v", got, center)
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
			}, manager.VimStyle{
				FontSize: 10,
				Badge:    manager.VimBadge{FontSize: 10, PaddingX: 4, PaddingY: 4},
			}, measureEightPerRune)

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
		}, manager.VimStyle{FontSize: 10, ModeColor: "#336699"}, measureEightPerRune)

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
