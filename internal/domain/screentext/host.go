package screentext

import (
	"errors"
	"maps"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/y3owk1n/fude/vim"
)

var (
	// ErrReadOnly is why the screen's text cannot change.
	ErrReadOnly = errors.New("the screen is read only")
	// ErrNotACommand is what an ex line no one runs fails with.
	ErrNotACommand = errors.New("not a command")
	// ErrNoContainer is why Ctrl+W found nowhere to go.
	ErrNoContainer = errors.New("no container that way")
)

// Host is a vim.Host over screen text. The cursor is a byte offset and a yank
// goes to onYank. An edit is handed to OnApply, which makes it somewhere else,
// such as in the app on screen, and is made in the text only to keep the
// machine in step until the screen is read again.
type Host struct {
	vim.Defaults

	text   *Text
	cursor int
	marks  map[rune]int
	onYank func(string)

	// Ex runs an ex line the machine leaves to the host, such as ":q". Nil
	// refuses every one.
	Ex func(line string) error
	// OnApply takes the edits a command makes, and the command's keys. Nil
	// makes the text read only.
	OnApply func(edits []vim.Edit, label string) error
	// OnUndo takes u, and OnRedo takes <C-r>. Nil leaves nothing to undo.
	OnUndo, OnRedo func()
	// CanEdit, when set, can refuse every change, undo and redo included, as
	// when the keys that would make it cannot reach the app.
	CanEdit func() error
}

// NewHost puts the cursor at pos. onYank receives yanked text, and may be
// nil.
func NewHost(text *Text, pos int, onYank func(string)) *Host {
	return &Host{text: text, cursor: pos, marks: map[rune]int{}, onYank: onYank}
}

// Screen returns the text the host holds.
func (h *Host) Screen() *Text { return h.text }

// Replace swaps in text read again, with the cursor at pos. It drops the
// marks a user can jump to, such as the last jump and the visual marks,
// since their offsets point into the text read before. The machine's own
// marks stay.
func (h *Host) Replace(text *Text, pos int) {
	h.text, h.cursor = text, pos

	maps.DeleteFunc(h.marks, func(name rune, _ int) bool { return name < vim.OwnMarks })
}

// Unhandled moves between containers on a Ctrl+W key, and runs any other
// line the machine does not own through Ex.
func (h *Host) Unhandled(line string) error {
	if count, key, ok := windowKey(line); ok {
		return h.moveContainer(key, count)
	}

	if h.Ex == nil {
		return ErrNotACommand
	}

	return h.Ex(line)
}

// windowKey reads a Ctrl+W line, such as "2<C-w>l", as its count and key.
func windowKey(line string) (int, string, bool) {
	prefix, key, ok := strings.Cut(line, "<C-w>")
	if !ok {
		return 0, "", false
	}

	count, err := strconv.Atoi(prefix)
	if err != nil {
		count = 1
	}

	return count, key, true
}

// Text returns the text the host holds.
func (h *Host) Text() vim.Text { return h.text }

// Cursor returns the cursor's byte offset.
func (h *Host) Cursor() int { return h.cursor }

// SetCursor moves the cursor.
func (h *Host) SetCursor(pos int) { h.cursor = pos }

// Apply hands the edits to OnApply, and makes them in the text it holds so
// the machine's reading of the text after them is right until the screen is
// read again.
func (h *Host) Apply(edits []vim.Edit, label string) error {
	err := h.Editable()
	if err != nil {
		return err
	}

	err = h.OnApply(edits, label)
	if err != nil {
		return err
	}

	h.text = h.text.Edit(edits)

	return nil
}

// Editable refuses insert and replace mode when no OnApply makes edits, or
// when CanEdit refuses.
func (h *Host) Editable() error {
	if h.OnApply == nil {
		return ErrReadOnly
	}

	if h.CanEdit != nil {
		return h.CanEdit()
	}

	return nil
}

// Undo hands u to OnUndo and leaves the cursor where it is.
func (h *Host) Undo() (int, int, error) {
	if h.OnUndo == nil {
		return 0, 0, vim.ErrNoStep
	}

	if h.CanEdit != nil {
		err := h.CanEdit()
		if err != nil {
			return 0, 0, err
		}
	}

	h.OnUndo()

	return h.cursor, h.cursor, nil
}

// Redo hands <C-r> to OnRedo and leaves the cursor where it is.
func (h *Host) Redo() (int, int, error) {
	if h.OnRedo == nil {
		return 0, 0, vim.ErrNoStep
	}

	if h.CanEdit != nil {
		err := h.CanEdit()
		if err != nil {
			return 0, 0, err
		}
	}

	h.OnRedo()

	return h.cursor, h.cursor, nil
}

// SetMark keeps a mark, except a lettered one, which the next scroll or edit
// would leave on other text. So m sets nothing and ' finds no mark.
func (h *Host) SetMark(name rune, pos int) {
	if name < vim.OwnMarks && unicode.IsLetter(name) {
		return
	}

	h.marks[name] = pos
}

// Mark returns a mark. A mark past the end of the text, left by a read of a
// longer screen, is unset.
func (h *Host) Mark(name rune) (int, bool) {
	pos, ok := h.marks[name]
	if !ok || pos > h.text.Len() {
		return 0, false
	}

	return pos, true
}

// DeleteMark unsets a mark.
func (h *Host) DeleteMark(name rune) { delete(h.marks, name) }

// Viewport shows every line, unwrapped.
func (h *Host) Viewport() (int, int, int) { return 0, h.text.Lines(), math.MaxInt }

// SetTop does nothing, since every line shows.
func (h *Host) SetTop(int) {}

// Clipboard is empty, since the screen pastes nowhere.
func (h *Host) Clipboard() (string, error) { return "", nil }

// SetClipboard hands yanked text to onYank.
func (h *Host) SetClipboard(text string) error {
	if h.onYank != nil {
		h.onYank(text)
	}

	return nil
}

// moveContainer moves the cursor count containers in the direction key
// names, as Ctrl+W moves between windows: h, j, k and l target the nearest
// container that way, and w and W target the next and the previous in reading
// order. The cursor lands on the character of that container nearest where it
// was on screen.
func (h *Host) moveContainer(key string, count int) error {
	from, ok := h.text.ContainerAt(h.cursor)
	if !ok || len(h.text.containers) < 2 {
		return ErrNoContainer
	}

	point, _ := h.text.Point(h.cursor)
	target := from

	for range max(count, 1) {
		next, found := h.text.neighbor(target, key)
		if !found {
			break
		}

		target = next
	}

	if target == from {
		return ErrNoContainer
	}

	h.cursor = h.text.NearestIn(target, point)

	return nil
}

// NewMachine returns a vim machine over host, set up for the screen. Search
// ignores case unless the pattern has a capital, and a yank goes to the
// clipboard.
func NewMachine(host *Host) *vim.Machine {
	machine := vim.New(host)
	machine.IgnoreCase(true, true)
	machine.UseClipboard(true)
	machine.Object("ic", containerObject(false))
	machine.Object("ac", containerObject(true))

	// ]i and [i go to the next and the previous icon, as [[ and ]] go to
	// sections.
	for name, back := range map[string]bool{"nexticon": false, "previcon": true} {
		machine.Command(name, func(_ vim.Text, pos, count int) ([]vim.Edit, string) {
			if target, ok := host.text.IconAfter(pos, count, back); ok {
				host.SetCursor(target)
			}

			return nil, ""
		})
	}

	machine.Map(vim.Normal, "]i", "<Plug>(nexticon)")
	machine.Map(vim.Normal, "[i", "<Plug>(previcon)")

	return machine
}

// containerObject finds the container under the cursor as a text object: its
// text, and with around the blank line after it too.
func containerObject(around bool) func(vim.Text, int, int) (int, int, bool) {
	return func(text vim.Text, start, _ int) (int, int, bool) {
		screen, found := text.(*Text)
		if !found {
			return 0, 0, false
		}

		index, found := screen.ContainerAt(start)
		if !found {
			return 0, 0, false
		}

		container := screen.containers[index]
		if !around {
			return container.Start, container.End, true
		}

		if index+1 < len(screen.containers) {
			return container.Start, screen.containers[index+1].Start, true
		}

		return container.Start, screen.Len(), true
	}
}
