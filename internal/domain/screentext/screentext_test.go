package screentext_test

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/fude/vim"

	"github.com/y3owk1n/neru/internal/domain/screentext"
)

// wordSave is a word several screens share.
const wordSave = "Save"

// screen is two lines read out of order: "File Edit View" over "Open Save".
func screen() []screentext.Word {
	return []screentext.Word{
		{Text: "View", Bounds: image.Rect(120, 10, 160, 30)},
		{Text: wordSave, Bounds: image.Rect(80, 52, 120, 70)},
		{Text: "File", Bounds: image.Rect(0, 10, 40, 30)},
		{Text: "Open", Bounds: image.Rect(0, 50, 40, 70)},
		{Text: "Edit", Bounds: image.Rect(60, 12, 100, 32)},
	}
}

func TestNew_LaysWordsOutInTheirColumns(t *testing.T) {
	got := screentext.New(screen()).String()
	if want := "File  Edit  View\nOpen    Save"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func TestNew_KeepsIndentationAndParagraphs(t *testing.T) {
	got := screentext.New([]screentext.Word{
		{Text: "if", Bounds: image.Rect(0, 0, 20, 10)},
		{Text: "go", Bounds: image.Rect(40, 12, 60, 22)},
		{Text: "end", Bounds: image.Rect(0, 40, 30, 50)},
	}).String()

	if want := "if\n    go\n\nend"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func TestHost_J_KeepsTheScreenColumn(t *testing.T) {
	text := screentext.New([]screentext.Word{
		{Text: "abcdef", Bounds: image.Rect(0, 0, 60, 10)},
		{Text: "xyz", Bounds: image.Rect(30, 12, 60, 22)},
	})
	host := screentext.NewHost(text, 4, nil)
	m := screentext.NewMachine(host)

	err := m.Feed("j")
	if err != nil {
		t.Fatal(err)
	}

	if got, _ := text.Point(host.Cursor()); got != image.Pt(45, 17) {
		t.Fatalf("j from e landed at %v, want the y below it", got)
	}
}

func TestHost_Marks_AreNeverKeptAcrossAReadOrByLetter(t *testing.T) {
	text := screentext.New(screen())
	host := screentext.NewHost(text, 0, nil)
	machine := screentext.NewMachine(host)

	for _, key := range []vim.Key{"m", "a", "w", "'", "a"} {
		_ = machine.Feed(key)
	}

	if got := machine.Message(); got != "mark not set" {
		t.Fatalf("'a after ma says %q, want mark not set", got)
	}

	// G sets '', which a read of the screen drops.
	_ = machine.Feed("G")

	host.Replace(text, 0)

	for _, key := range []vim.Key{"'", "'"} {
		_ = machine.Feed(key)
	}

	if got := machine.Message(); got != "mark not set" {
		t.Fatalf("'' after a read says %q, want mark not set", got)
	}
}

func TestHost_Keys_MoveTheCursorOverTheScreen(t *testing.T) {
	tests := []struct {
		name  string
		keys  string
		point image.Point
	}{
		{name: "w goes to the next word", keys: "w", point: image.Pt(65, 22)},
		{name: "e goes to the end of the word", keys: "e", point: image.Pt(35, 20)},
		{name: "j goes to the line below", keys: "j", point: image.Pt(5, 60)},
		{name: "search goes to the match", keys: "/sav<CR>", point: image.Pt(85, 61)},
		{name: "count repeats a motion", keys: "2w", point: image.Pt(125, 20)},
		{name: "G goes to the last line", keys: "G", point: image.Pt(5, 60)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := screentext.New(screen())
			host := screentext.NewHost(text, 0, nil)
			m := screentext.NewMachine(host)

			for _, k := range vim.Keys(test.keys) {
				err := m.Feed(k)
				if err != nil {
					t.Fatalf("feed %q: %v", k, err)
				}
			}

			got, _ := text.Point(host.Cursor())
			if got != test.point {
				t.Fatalf("cursor at %v, want %v", got, test.point)
			}
		})
	}
}

func TestHost_Yank_CopiesTheWord(t *testing.T) {
	var copied string

	text := screentext.New(screen())
	host := screentext.NewHost(text, 0, func(s string) { copied = s })
	m := screentext.NewMachine(host)

	for _, k := range vim.Keys("wyiw") {
		err := m.Feed(k)
		if err != nil {
			t.Fatalf("feed %q: %v", k, err)
		}
	}

	if copied != "Edit" {
		t.Fatalf("copied %q, want %q", copied, "Edit")
	}
}

func TestText_Nearest_FindsTheByteUnderAPoint(t *testing.T) {
	text := screentext.New(screen())

	if got := text.Slice(
		text.Nearest(image.Pt(102, 62)),
		text.Nearest(image.Pt(102, 62))+1,
	); got != "v" {
		t.Fatalf("nearest byte is %q, want %q", got, "v")
	}
}

func TestKey_TranslatesTapKeysToVimNotation(t *testing.T) {
	tests := []struct {
		key  string
		want vim.Key
		ok   bool
	}{
		{key: "w", want: "w", ok: true},
		{key: "Escape", want: "<Esc>", ok: true},
		{key: "Return", want: "<CR>", ok: true},
		{key: "Delete", want: "<BS>", ok: true},
		{key: "Space", want: " ", ok: true},
		{key: "Shift+g", want: "G", ok: true},
		{key: "Shift+4", want: "$", ok: true},
		{key: "Shift+;", want: ":", ok: true},
		{key: "Ctrl+D", want: "<C-d>", ok: true},
		{key: "shift+v", want: "V", ok: true},
		{key: "shift+4", want: "$", ok: true},
		{key: "ctrl+d", want: "<C-d>", ok: true},
		{key: "Cmd+C", ok: false},
	}

	for _, tt := range tests {
		got, ok := screentext.Key(tt.key)
		if got != tt.want || ok != tt.ok {
			t.Errorf("Key(%q) = %q, %v, want %q, %v", tt.key, got, ok, tt.want, tt.ok)
		}
	}
}

func TestFeedKeys_SpellsTextAsKeys(t *testing.T) {
	got, ok := screentext.FeedKeys("Hi $x\n")
	want := []string{"H", "i", "Space", "Shift+4", "x", "Return"}

	if !ok || !slices.Equal(got, want) {
		t.Fatalf("FeedKeys = %v, %v, want %v, true", got, ok, want)
	}

	if _, ok := screentext.FeedKeys("café"); ok {
		t.Fatal("FeedKeys typed a rune with no key on a US layout")
	}
}

func TestNew_UsesTheBoxEachRuneWasReadFrom(t *testing.T) {
	text := screentext.New([]screentext.Word{{
		Text:   "ab",
		Bounds: image.Rect(0, 0, 30, 10),
		Boxes:  []image.Rectangle{image.Rect(0, 0, 5, 10), image.Rect(20, 0, 30, 10)},
	}})

	if got, _ := text.Box(1); got != image.Rect(20, 0, 30, 10) {
		t.Fatalf("box of b = %v, want where it was read", got)
	}
}

func TestHost_Delete_HandsTheEditOverAndKeepsTheTextInStep(t *testing.T) {
	var got []vim.Edit

	text := screentext.New(screen())
	host := screentext.NewHost(text, 0, nil)
	host.OnApply = func(edits []vim.Edit, _ string) error {
		got = append(got, edits...)

		return nil
	}

	m := screentext.NewMachine(host)
	for _, k := range vim.Keys("jdd") {
		err := m.Feed(k)
		if err != nil {
			t.Fatalf("feed %q: %v", k, err)
		}
	}

	if len(got) != 1 {
		t.Fatalf("edits handed over = %v, want one", got)
	}

	if s := host.Screen().String(); s != "File  Edit  View" {
		t.Fatalf("text after dd = %q, want the line gone", s)
	}
}

func TestNew_PaddingFillsTheGapOnScreen(t *testing.T) {
	text := screentext.New([]screentext.Word{
		{Text: "ab", Bounds: image.Rect(0, 0, 20, 10)},
		{Text: "cd", Bounds: image.Rect(60, 0, 80, 10)},
	})

	first, _ := text.Box(2)
	last, _ := text.Box(strings.Index(text.String(), "c") - 1)

	if first.Min.X != 20 || last.Max.X != 60 {
		t.Fatalf("padding runs %d..%d, want the gap 20..60", first.Min.X, last.Max.X)
	}
}

func TestText_AppChars_LeavesOutWhereALineStarts(t *testing.T) {
	text := screentext.New([]screentext.Word{
		{Text: "if", Bounds: image.Rect(0, 0, 20, 10)},
		{Text: "go", Bounds: image.Rect(40, 12, 60, 22)},
	})

	if got := text.AppChars(0, text.Len()); got != len("if\ngo") {
		t.Fatalf("AppChars = %d, want %d", got, len("if\ngo"))
	}
}

func TestText_AppChars_LeavesOutTheIconALineStartsWith(t *testing.T) {
	// The app has no text for the icon or the gap after it, so a selection
	// from where the app's text starts takes Save and its line break only.
	text := screentext.New([]screentext.Word{
		screentext.Icon(image.Rect(0, 0, 10, 10)),
		{Text: "Save", Bounds: image.Rect(20, 0, 60, 10)},
		{Text: "Next", Bounds: image.Rect(0, 12, 40, 22)},
	})

	next := strings.Index(text.String(), "Next")
	if got := text.AppChars(0, next); got != len("Save\n") {
		t.Fatalf("AppChars over %q = %d, want %d", text.String(), got, len("Save\n"))
	}
}

func TestText_Edit_KeepsTheSpacingAndTheIcons(t *testing.T) {
	// An edited text stands in for the screen until it is read again, so
	// the gap after the edit still counts as the one space the app has, and
	// the icon is still no text of the app's.
	text := screentext.New([]screentext.Word{
		screentext.Icon(image.Rect(0, 0, 10, 10)),
		{Text: "save", Bounds: image.Rect(20, 0, 60, 10)},
		{Text: "file", Bounds: image.Rect(70, 0, 110, 10)},
	})

	save := strings.Index(text.String(), "save")
	edited := text.Edit([]vim.Edit{{Start: save, End: save + 1, Text: "S"}})

	if got, want := edited.AppText(0, edited.Len()), "Save file"; got != want {
		t.Fatalf("AppText of %q = %q, want %q", edited.String(), got, want)
	}

	if got, want := edited.AppChars(save, edited.Len()), len("Save file"); got != want {
		t.Fatalf("AppChars of %q = %d, want %d", edited.String(), got, want)
	}
}

func TestText_AppChars_CountsAGapAsTheSpacesItHolds(t *testing.T) {
	// Characters 10.4 pixels apart read as 10 wide, so by the third word the
	// columns from the left edge have drifted a character, and the layout
	// pads with a space the app does not have.
	text := screentext.New([]screentext.Word{
		{Text: "aaaaaaaaaa", Bounds: image.Rect(0, 0, 104, 10)},
		{Text: "bbbbbbbbbb", Bounds: image.Rect(114, 0, 218, 10)},
		{Text: "cccccccccc", Bounds: image.Rect(229, 0, 333, 10)},
	})

	want := "aaaaaaaaaa bbbbbbbbbb cccccccccc"
	if got := text.AppChars(0, text.Len()); got != len(want) {
		t.Fatalf("AppChars = %d over %q, want %d", got, text.String(), len(want))
	}

	if got := text.AppText(0, text.Len()); got != want {
		t.Fatalf("AppText = %q, want %q", got, want)
	}
}

// columns is two columns side by side, a gutter apart, and a footer below
// both on another background.
func columns() []screentext.Word {
	const page, footer = 0xF0F0F0, 0x303030

	return []screentext.Word{
		{Text: "Menu", Bounds: image.Rect(0, 0, 40, 10), Background: page},
		{Text: "Home", Bounds: image.Rect(0, 12, 40, 22), Background: page},
		{Text: "Article", Bounds: image.Rect(200, 0, 270, 10), Background: page},
		{Text: "body", Bounds: image.Rect(200, 12, 240, 22), Background: page},
		{Text: "Footer", Bounds: image.Rect(0, 30, 60, 40), Background: footer},
		{Text: "about", Bounds: image.Rect(70, 30, 120, 40), Background: footer},
		{Text: "terms", Bounds: image.Rect(130, 30, 180, 40), Background: footer},
		{Text: "privacy", Bounds: image.Rect(190, 30, 260, 40), Background: footer},
	}
}

// wordFour is a four-character word the row tests lay out.
const wordFour = "aaaa"

// row is words of reader line line, each five pixels a character and ten
// tall, starting at the x positions given, on the top row.
func row(line int, xs ...int) []screentext.Word {
	words := make([]screentext.Word, 0, len(xs))
	for _, x := range xs {
		words = append(words, screentext.Word{
			Text: wordFour, Bounds: image.Rect(x, 0, x+20, 10), Line: line,
		})
	}

	return words
}

// wordTodo is a task line's first word in the border cases.
const wordTodo = "todo"

func TestNew_GroupsWordsIntoContainers(t *testing.T) {
	const page, other = 0xF0F0F0, 0x303030

	word := func(text string, bounds image.Rectangle, background uint32) screentext.Word {
		return screentext.Word{Text: text, Bounds: bounds, Background: background}
	}

	tests := []struct {
		name  string
		words []screentext.Word
		want  int
	}{
		{
			name: "a word a paragraph break below and off to the side stays apart",
			words: []screentext.Word{
				word("Logo", image.Rect(0, 0, 40, 10), 0),
				word("Text", image.Rect(50, 25, 90, 35), 0),
			},
			want: 2,
		},
		{
			name: "a line stays apart from the one under it across a border below it",
			words: []screentext.Word{
				{Text: "Tasks", Bounds: image.Rect(0, 0, 40, 10), BorderBelow: 2},
				{Text: wordTodo, Bounds: image.Rect(0, 14, 40, 24)},
			},
			want: 2,
		},
		{
			name: "a line stays apart from the one over it across a border above it",
			words: []screentext.Word{
				{Text: "Tasks", Bounds: image.Rect(0, 0, 40, 10)},
				{Text: wordTodo, Bounds: image.Rect(0, 14, 40, 24), BorderAbove: 2},
			},
			want: 2,
		},
		{
			name: "a border past the line above leaves two lines in one box together",
			words: []screentext.Word{
				{Text: wordTodo, Bounds: image.Rect(0, 30, 40, 40)},
				{Text: "done", Bounds: image.Rect(0, 44, 40, 54), BorderAbove: 20},
			},
			want: 1,
		},
		{
			name: "a tall heading joins the text a paragraph break under it",
			words: []screentext.Word{
				word("Title", image.Rect(0, 0, 50, 30), 0),
				word("text", image.Rect(0, 45, 40, 55), 0),
			},
			want: 1,
		},
		{
			name: "text a paragraph break above a heading stays apart from it",
			words: []screentext.Word{
				word("menu", image.Rect(0, 0, 40, 10), 0),
				word("Title", image.Rect(0, 20, 50, 50), 0),
			},
			want: 2,
		},
		{
			name:  "two evenly spaced lines on one row stay apart across a gutter",
			words: slices.Concat(row(1, 0, 30, 60, 90), row(2, 140, 170, 200, 230)),
			want:  2,
		},
		{
			name:  "two short lines on one row join across the same gap",
			words: slices.Concat(row(1, 0, 30), row(2, 65, 95)),
			want:  1,
		},
		{
			name: "a line cannot reach past the line under it to a heading",
			words: []screentext.Word{
				word("Quote", image.Rect(60, 0, 110, 18), 0),
				word("middleline", image.Rect(60, 22, 200, 38), 0),
				word("Title", image.Rect(0, 60, 90, 87), 0),
			},
			want: 2,
		},
		{
			name: "the end of an evenly spaced line does not step down across a gutter",
			words: slices.Concat(row(1, 0, 25, 50, 75), []screentext.Word{
				{Text: wordFour, Bounds: image.Rect(125, 14, 145, 24), Line: 2},
				{Text: wordFour, Bounds: image.Rect(150, 14, 170, 24), Line: 2},
			}),
			want: 2,
		},
		{
			// A reader that measures ink, as Windows's does, gives a row of
			// short words a short box. A table's rows are still one column.
			name: "rows a reader measured at their ink join as rows of full lines do",
			words: []screentext.Word{
				{Text: wordFour, Bounds: image.Rect(0, 0, 20, 9), Line: 1},
				{Text: wordFour, Bounds: image.Rect(0, 30, 20, 39), Line: 2},
				{Text: wordFour, Bounds: image.Rect(0, 60, 20, 69), Line: 3},
			},
			want: 1,
		},
		{
			// A reader can split a widely justified line into short ones,
			// which have no spacing of their own to measure.
			name:  "a short line on the row of an evenly spaced one stays apart across a gutter",
			words: slices.Concat(row(1, 0, 25, 50, 75), row(2, 110)),
			want:  2,
		},
		{
			name: "two blocks whose corners touch a paragraph break apart stay apart",
			words: []screentext.Word{
				word("upper", image.Rect(100, 0, 150, 18), 0),
				word("lowerline", image.Rect(0, 40, 103, 58), 0),
			},
			want: 2,
		},
		{
			name: "icons between two blocks do not join them",
			words: []screentext.Word{
				word("aaaa", image.Rect(0, 0, 40, 10), 0),
				screentext.Icon(image.Rect(0, 14, 12, 26)),
				screentext.Icon(image.Rect(0, 28, 12, 36)),
				word("bbbb", image.Rect(0, 40, 40, 50), 0),
			},
			want: 3,
		},
		{
			name: "an icon joins the label beside it",
			words: []screentext.Word{
				screentext.Icon(image.Rect(0, 0, 10, 10)),
				word("Save", image.Rect(14, 0, 54, 10), 0),
			},
			want: 1,
		},
		{
			name: "a word on another background inside a paragraph is part of it",
			words: []screentext.Word{
				word("the", image.Rect(0, 0, 30, 10), page),
				word("code", image.Rect(40, 0, 80, 10), other),
				word("here", image.Rect(90, 0, 130, 10), page),
				word("more", image.Rect(0, 12, 40, 22), page),
				word("words", image.Rect(50, 12, 100, 22), page),
				word("below", image.Rect(110, 12, 160, 22), page),
			},
			want: 1,
		},
		{
			name: "a block on another background barely overlapping stays apart",
			words: []screentext.Word{
				word("aaaa", image.Rect(0, 0, 40, 10), page),
				word("bbbbbbbbbb", image.Rect(0, 12, 100, 22), page),
				word("cccc", image.Rect(95, 20, 135, 30), other),
			},
			want: 2,
		},
		{
			// One and Six are too far apart to join without Two.
			name: "a list's highlighted item joins the items around it",
			words: []screentext.Word{
				word("One", image.Rect(0, 0, 30, 10), page),
				word("Two", image.Rect(0, 18, 30, 28), other),
				word("Six", image.Rect(0, 36, 30, 46), page),
			},
			want: 1,
		},
		{
			name: "rows striped in two backgrounds are one table",
			words: []screentext.Word{
				word("rowa", image.Rect(0, 0, 40, 10), page),
				word("rowb", image.Rect(0, 18, 40, 28), other),
				word("rowc", image.Rect(0, 36, 40, 46), page),
				word("rowd", image.Rect(0, 54, 40, 64), other),
			},
			want: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := screentext.New(test.words).Containers(); len(got) != test.want {
				t.Fatalf("%d containers %v, want %d", len(got), got, test.want)
			}
		})
	}
}

func TestNew_LaysEachContainerOutAsItsOwnBlock(t *testing.T) {
	got := screentext.New(columns()).String()
	if want := "Menu\nHome\n\nArticle\nbody\n\nFooter about terms privacy"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

// numberedLines is an editor's text beside its line numbers, a few
// characters from it, on one background.
func numberedLines(numbers ...string) []screentext.Word {
	texts := []string{"Why do", "It is", "content"}
	words := make([]screentext.Word, 0, len(numbers)*2)

	for line, number := range numbers {
		top := line * 20
		words = append(words,
			screentext.Word{Text: number, Bounds: image.Rect(8, top, 15, top+10)},
			screentext.Word{Text: texts[line%len(texts)], Bounds: image.Rect(44, top, 100, top+10)},
		)
	}

	return words
}

func TestNew_KeepsALineNumberGutterOutOfTheText(t *testing.T) {
	got := screentext.New(numberedLines("1", "2", "3")).String()
	if want := "1\n2\n3\n\nWhy do\nIt is\ncontent"; got != want {
		t.Fatalf("text = %q, want the gutter apart from the text, %q", got, want)
	}
}

func TestNew_KeepsTheOneReadableLineNumberOutOfTheText(t *testing.T) {
	// An editor dims every number but the current line's, which is all that
	// was read.
	words := numberedLines("", "2", "")
	words = slices.DeleteFunc(words, func(word screentext.Word) bool { return word.Text == "" })

	got := screentext.New(words).String()
	if strings.Contains(got, "2 ") || strings.HasPrefix(got, "Why do\n2") {
		t.Fatalf("text = %q, want the number apart from its line", got)
	}
}

func TestNew_KeepsANumberThatStartsAShortGap(t *testing.T) {
	got := screentext.New([]screentext.Word{
		{Text: "3", Bounds: image.Rect(0, 0, 7, 10)},
		{Text: "apples", Bounds: image.Rect(14, 0, 56, 10)},
		{Text: "pears", Bounds: image.Rect(14, 20, 49, 30)},
		{Text: "plums", Bounds: image.Rect(14, 40, 49, 50)},
	}).String()

	if want := "3 apples\n  pears\n  plums"; got != want {
		t.Fatalf("text = %q, want the number kept on its line, %q", got, want)
	}
}

func TestHost_CtrlW_MovesBetweenContainers(t *testing.T) {
	tests := []struct {
		name  string
		keys  string
		point image.Point
	}{
		{name: "l goes to the container on the right", keys: "j<C-w>l", point: image.Pt(205, 17)},
		{name: "j goes to the container below", keys: keysDownW, point: image.Pt(5, 35)},
		{name: "w goes to the next in reading order", keys: "<C-w>w", point: image.Pt(205, 5)},
		{
			name:  "h passes over a container below for the one beside",
			keys:  "<C-w>w<C-w>h",
			point: image.Pt(35, 5),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := screentext.New(columns())
			host := screentext.NewHost(text, 0, nil)
			machine := screentext.NewMachine(host)

			for _, key := range vim.Keys(test.keys) {
				err := machine.Feed(key)
				if err != nil {
					t.Fatalf("feed %q: %v", key, err)
				}
			}

			if got, _ := text.Point(host.Cursor()); got != test.point {
				t.Fatalf("cursor at %v, want %v", got, test.point)
			}
		})
	}
}

func TestHost_CtrlW_ReachesAContainerInsideTheBoundsOfAnother(t *testing.T) {
	// The line under the left column runs on under the right one, so the
	// left container's bounds take in the right one's.
	text := screentext.New([]screentext.Word{
		{Text: "Left", Bounds: image.Rect(0, 0, 40, 10)},
		{Text: "item", Bounds: image.Rect(0, 12, 40, 22)},
		{Text: "more", Bounds: image.Rect(0, 24, 40, 34)},
		{Text: "Side", Bounds: image.Rect(200, 0, 240, 10)},
		{Text: wordFour, Bounds: image.Rect(0, 40, 40, 50)},
		{Text: "bbbb", Bounds: image.Rect(50, 40, 90, 50)},
		{Text: "cccc", Bounds: image.Rect(100, 40, 140, 50)},
		{Text: "dddd", Bounds: image.Rect(150, 40, 190, 50)},
		{Text: "eeee", Bounds: image.Rect(200, 40, 240, 50)},
	})
	host := screentext.NewHost(text, 0, nil)
	machine := screentext.NewMachine(host)

	for _, key := range vim.Keys("<C-w>l") {
		err := machine.Feed(key)
		if err != nil {
			t.Fatalf("feed %q: %v", key, err)
		}
	}

	if got, _ := text.Point(host.Cursor()); got != image.Pt(205, 5) {
		t.Fatalf("cursor at %v, want on Side at (205,5)", got)
	}
}

// The words and keys the column tests share.
const (
	wordLone  = "Lone"
	wordLtwo  = "Ltwo"
	keysDownW = "<C-w>j"
)

// sections is two columns of two sections each, the right column's a little
// lower, so by height alone their sections interleave.
func sections() []screentext.Word {
	return []screentext.Word{
		{Text: wordLone, Bounds: image.Rect(0, 0, 40, 10)},
		{Text: wordLtwo, Bounds: image.Rect(0, 40, 40, 50)},
		{Text: "Rone", Bounds: image.Rect(200, 20, 240, 30)},
		{Text: "Rtwo", Bounds: image.Rect(200, 52, 240, 62)},
	}
}

func TestNew_LaysOutAColumnBeforeTheColumnBesideIt(t *testing.T) {
	got := screentext.New(sections()).String()
	if want := "Lone\n\nLtwo\n\nRone\n\nRtwo"; got != want {
		t.Fatalf("text = %q, want each column whole, %q", got, want)
	}
}

func TestHost_CtrlW_StaysInTheColumnUntilItEnds(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		keys    string
		want    image.Point
		refused bool
	}{
		{
			name: "j goes to the section below",
			from: wordLone,
			keys: keysDownW,
			want: image.Pt(5, 45),
		},
		{
			name:    "j at the column's end goes nowhere",
			from:    wordLtwo,
			keys:    keysDownW,
			refused: true,
		},
		{
			name: "l goes to the column beside",
			from: wordLtwo,
			keys: "<C-w>l",
			want: image.Pt(205, 57),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := screentext.New(sections())
			host := screentext.NewHost(text, strings.Index(text.String(), test.from), nil)
			machine := screentext.NewMachine(host)

			var err error
			for _, key := range vim.Keys(test.keys) {
				err = machine.Feed(key)
			}

			if test.refused {
				if err == nil {
					t.Fatalf("moved to %v, want no container that way", host.Cursor())
				}

				return
			}

			if got, _ := text.Point(host.Cursor()); err != nil || got != test.want {
				t.Fatalf("cursor at %v (%v), want %v", got, err, test.want)
			}
		})
	}
}

func TestHost_YankInsideContainer_ACountTakesTheBlockAround(t *testing.T) {
	tests := []struct {
		keys string
		want string
	}{
		{keys: "yic", want: wordLone},
		{keys: "y2ic", want: "Lone\n\nLtwo"},
		{keys: "y3ic", want: "Lone\n\nLtwo\n\nRone\n\nRtwo"},
	}

	for _, test := range tests {
		t.Run(test.keys, func(t *testing.T) {
			var copied string

			text := screentext.New(sections())
			host := screentext.NewHost(text, 0, func(s string) { copied = s })
			machine := screentext.NewMachine(host)

			for _, key := range vim.Keys(test.keys) {
				err := machine.Feed(key)
				if err != nil {
					t.Fatalf("feed %q: %v", key, err)
				}
			}

			if copied != test.want {
				t.Fatalf("copied %q, want %q", copied, test.want)
			}
		})
	}
}

func TestHost_YankInsideContainer_CopiesItsText(t *testing.T) {
	var copied string

	text := screentext.New(columns())
	host := screentext.NewHost(
		text,
		strings.Index(text.String(), "body"),
		func(s string) { copied = s },
	)
	machine := screentext.NewMachine(host)

	for _, key := range vim.Keys("yic") {
		err := machine.Feed(key)
		if err != nil {
			t.Fatalf("feed %q: %v", key, err)
		}
	}

	if copied != "Article\nbody" {
		t.Fatalf("copied %q, want the article's text", copied)
	}
}

func TestText_WideCharacters_TakeTwoCells(t *testing.T) {
	// Each narrow character is 10 wide, so 语 and 言 are 20 each.
	text := screentext.New([]screentext.Word{
		{Text: "Go语言", Bounds: image.Rect(100, 100, 160, 130)}, //nolint:gosmopolitan
		{Text: "test", Bounds: image.Rect(170, 100, 210, 130)},
	})

	if got, want := text.String(), "Go语言 test"; got != want { //nolint:gosmopolitan
		t.Fatalf("text = %q, want %q, with one space where the screen has one cell", got, want)
	}

	narrow, _ := text.Box(0)
	wide, _ := text.Box(len("Go"))

	if narrow.Dx() != 10 || wide.Dx() != 20 {
		t.Fatalf("G is %d wide and the first wide one %d, want 10 and 20", narrow.Dx(), wide.Dx())
	}
}

func TestText_AppChars_CountsAVowelSignWithItsConsonant(t *testing.T) {
	text := screentext.New([]screentext.Word{{Text: "กิน", Bounds: image.Rect(100, 100, 130, 130)}})

	if got := text.AppChars(0, text.Len()); got != 2 {
		t.Fatalf(
			"AppChars(%q) = %d, want 2, since กิ is one character to the app",
			text.String(),
			got,
		)
	}
}

func TestText_RightToLeftWord_StartsAtTheRight(t *testing.T) {
	text := screentext.New(
		[]screentext.Word{{Text: "שלום", Bounds: image.Rect(100, 100, 140, 130)}},
	)

	first, _ := text.Box(0)
	if first != image.Rect(130, 100, 140, 130) {
		t.Fatalf("the first letter of %q is at %v, want the right end", text.String(), first)
	}
}

func TestText_AppStart_SkipsPaddingAndIcons(t *testing.T) {
	text := screentext.New([]screentext.Word{
		{Text: "Title", Bounds: image.Rect(100, 100, 175, 130)},
		screentext.Icon(image.Rect(160, 140, 175, 170)),
		{Text: "search", Bounds: image.Rect(190, 140, 280, 170)},
	})

	search := strings.Index(text.String(), "search")
	tests := []struct {
		name string
		pos  int
		want int
	}{
		{"a character stays", 1, 1},
		{
			"the line start goes past padding and the icon",
			strings.Index(text.String(), "\n") + 1,
			search,
		},
		{"a character after the text starts stays", search + 2, search + 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := text.AppStart(tt.pos); got != tt.want {
				t.Fatalf("AppStart(%d) = %d, want %d in %q", tt.pos, got, tt.want, text.String())
			}
		})
	}
}

func TestText_Icons_AreReachedButNotCopied(t *testing.T) {
	text := screentext.New([]screentext.Word{
		{Text: wordSave, Bounds: image.Rect(0, 0, 40, 10)},
		screentext.Icon(image.Rect(50, 0, 60, 10)),
		{Text: "Close", Bounds: image.Rect(70, 0, 120, 10)},
	})
	host := screentext.NewHost(text, 0, nil)
	machine := screentext.NewMachine(host)

	for _, key := range vim.Keys("w") {
		err := machine.Feed(key)
		if err != nil {
			t.Fatalf("feed %q: %v", key, err)
		}
	}

	if got, _ := text.Point(host.Cursor()); got != image.Pt(55, 5) {
		t.Fatalf("w landed at %v, want the icon at (55,5)", got)
	}

	if got := text.AppText(0, text.Len()); got != "Save Close" {
		t.Fatalf("AppText = %q, want the line without the icon", got)
	}

	if got := text.AppChars(0, text.Len()); got != len("Save  Close") {
		t.Fatalf("AppChars = %d, want %d", got, len("Save  Close"))
	}
}

func TestHost_BracketI_MovesBetweenIcons(t *testing.T) {
	tests := []struct {
		name  string
		keys  string
		point image.Point
	}{
		{name: "]i goes to the next icon", keys: "]i", point: image.Pt(55, 5)},
		{name: "a count skips icons", keys: "2]i", point: image.Pt(135, 5)},
		{name: "[i goes back to the previous icon", keys: "$[i", point: image.Pt(55, 5)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := screentext.New([]screentext.Word{
				{Text: wordSave, Bounds: image.Rect(0, 0, 40, 10)},
				screentext.Icon(image.Rect(50, 0, 60, 10)),
				{Text: "Close", Bounds: image.Rect(70, 0, 120, 10)},
				screentext.Icon(image.Rect(130, 0, 140, 10)),
			})
			host := screentext.NewHost(text, 0, nil)
			machine := screentext.NewMachine(host)

			for _, key := range vim.Keys(test.keys) {
				err := machine.Feed(key)
				if err != nil {
					t.Fatalf("feed %q: %v", key, err)
				}
			}

			if got, _ := text.Point(host.Cursor()); got != test.point {
				t.Fatalf("cursor at %v, want %v", got, test.point)
			}
		})
	}
}
