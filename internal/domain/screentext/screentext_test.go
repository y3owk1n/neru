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

// columns is two columns side by side, a gutter apart, and a footer below on
// another background.
func columns() []screentext.Word {
	const page, footer = 0xF0F0F0, 0x303030

	return []screentext.Word{
		{Text: "Menu", Bounds: image.Rect(0, 0, 40, 10), Background: page},
		{Text: "Home", Bounds: image.Rect(0, 12, 40, 22), Background: page},
		{Text: "Article", Bounds: image.Rect(200, 0, 270, 10), Background: page},
		{Text: "body", Bounds: image.Rect(200, 12, 240, 22), Background: page},
		{Text: "Footer", Bounds: image.Rect(0, 30, 60, 40), Background: footer},
	}
}

func TestNew_LaysEachContainerOutAsItsOwnBlock(t *testing.T) {
	got := screentext.New(columns()).String()
	if want := "Menu\nHome\n\nArticle\nbody\n\nFooter"; got != want {
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
		{name: "j goes to the container below", keys: "<C-w>j", point: image.Pt(5, 35)},
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
