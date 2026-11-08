package modes

import (
	"cmp"
	"errors"
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/y3owk1n/fude/vim"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/screentext"
)

// vimReturn is Return in vim's notation.
const vimReturn vim.Key = "<CR>"

// vimControlKeys are the Ctrl chords vim reads, held back from the focused
// app while a session is open so they reach the machine, or while typing, so
// they reach the app through the key feed.
const vimControlKeys = "abdefgilnoprtuvwxy["

const (
	// vimMaxCountDigits is the longest count a key may extend. A longer one
	// would run a motion or a macro for minutes under h.mu, with the keyboard
	// held.
	vimMaxCountDigits = 4
	// The scroll keys' distances: line heights for Ctrl+E and Ctrl+Y, and
	// shares of the region read for the rest and for j and k past an edge.
	vimScrollLines    = 2
	vimScrollHalfPage = 0.5
	vimScrollPage     = 0.85
	vimScrollEdge     = 0.33
)

// vimChords lists the Ctrl chords an open session holds back, or none.
func (h *handlerState) vimChords() []string {
	if h.vim == nil {
		return nil
	}

	chords := make([]string, 0, len(vimControlKeys))
	for _, key := range vimControlKeys {
		chords = append(chords, "Ctrl+"+strings.ToUpper(string(key)))
	}

	return chords
}

// handleVimKey runs one key. With nothing pending, Escape leaves the mode,
// Return clicks the character under the cursor or drags across the visual
// selection, Ctrl+L reads the screen again, s starts a jump, and the scroll
// keys scroll the app. Everything else goes to the machine, and what it asks
// of the app is done once it returns.
func (h *handlerState) handleVimKey(key string) {
	session := h.vim

	if session.typing {
		h.handleVimTypingKey(key)

		return
	}

	vimKey, ok := screentext.Key(key)
	if !ok {
		return
	}

	if session.jump != nil {
		h.handleVimJumpKey(vimKey)

		return
	}

	h.handleVimMachineKey(vimKey)
}

// handleVimMachineKey runs a key in vim's notation: the keys vim mode owns,
// or the machine's, and what the machine asks of the app once it returns.
func (h *handlerState) handleVimMachineKey(vimKey vim.Key) {
	session := h.vim
	machine := session.machine

	if h.handleVimZKey(vimKey) {
		return
	}

	if vimKey == "." && session.lastInsert != nil && !session.repeating {
		if count, ok := vimPendingCount(machine); ok || machine.Idle() {
			if ok && !h.feedVim("<Esc>") {
				return
			}

			h.vimRepeatInsert(count)

			return
		}
	}

	if machine.Idle() {
		session.commandKeys = nil

		if h.handleVimIdleKey(vimKey) {
			return
		}
	}

	if machine.Mode() == vim.Visual && vimKey == vimReturn {
		if sel, ok := machine.Selection(); ok {
			h.vimDragRange(sel.Start, sel.End)
		}

		h.exitMode()

		return
	}

	if vimCountTooLong(machine, vimKey) {
		return
	}

	line := session.host.Screen().LineOf(session.host.Cursor())
	session.edited, session.edits, session.yank, session.notice = false, nil, "", ""
	session.commandKeys = append(session.commandKeys, vimKey)

	if !h.feedVim(vimKey) {
		return
	}

	if machine.Mode() == vim.Insert {
		session.insertCommand = slices.Clone(session.commandKeys)
	}

	if act := session.act; act != nil {
		session.act = nil
		act()

		return
	}

	if len(session.edits) > 0 {
		h.vimApplyEdits(session.edits, session.label)

		return
	}

	// A refused change copies nothing, and its notice says why.
	if session.yank != "" && session.notice == "" {
		h.vimYank(session.yank)
	}

	if machine.Mode() == vim.Insert {
		h.vimStartTyping(vimCaretIn(session.host.Screen(), session.host.Cursor()), true)

		return
	}

	if h.vimScrollAtEdge(vimKey, line) {
		return
	}

	h.followVimCursor()

	if !h.swapInRefinedVimText() {
		h.redrawFrame(h.vimFrame(), "update vim overlay")
	}
}

// vimInsert is a command that typed into the app, and what it typed.
type vimInsert struct {
	command []vim.Key
	typed   []string
}

// vimRepeatInsert repeats the last command that typed into the app, with
// what it typed, for . after such a command. The machine never saw the
// typing, so its own . would repeat the command alone. A count replaces the
// command's own, as 3. after ciw changes three words.
func (h *handlerState) vimRepeatInsert(count int) {
	session := h.vim
	last := session.lastInsert

	command := last.command
	if count > 0 {
		for len(command) > 0 && len(command[0]) == 1 && unicode.IsDigit(rune(command[0][0])) {
			command = command[1:]
		}

		command = slices.Concat(vimCountKeys(count), command)
	}

	// The replay runs the machine's own . for a . inside the command, rather
	// than this one again.
	session.repeating = true
	for _, key := range command {
		h.handleVimMachineKey(key)

		if h.vim != session {
			return
		}
	}

	session.repeating = false

	if !session.typing {
		return
	}

	if len(last.typed) > 0 {
		session.eraseOnEscape = false
	}

	h.vimFeed(last.typed...)
	session.typedKeys = last.typed

	h.handleVimTypingKey(configpkg.KeyNameEscape)
}

// vimPendingCount returns the count typed so far when a count is all the
// machine holds.
func vimPendingCount(machine *vim.Machine) (int, bool) {
	shown := machine.Shown()
	if shown == "" || strings.TrimFunc(shown, unicode.IsDigit) != "" ||
		machine.Mode() != vim.Normal || machine.Pending() {
		return 0, false
	}

	count, err := strconv.Atoi(shown)

	return count, err == nil
}

// vimCountKeys returns the keys that type count, or none for zero.
func vimCountKeys(count int) []vim.Key {
	if count <= 0 {
		return nil
	}

	keys := make([]vim.Key, 0, vimMaxCountDigits)
	for _, digit := range strconv.Itoa(count) {
		keys = append(keys, vim.Key(digit))
	}

	return keys
}

// handleVimZKey scrolls the app sideways for zh, zl, zH and zL, and reports
// whether it took key. A z with nothing or only a count before it waits for
// its second key. Any other second key goes to the machine with the z and
// the count.
func (h *handlerState) handleVimZKey(vimKey vim.Key) bool {
	session := h.vim
	machine := session.machine

	if session.zPrefix {
		session.zPrefix = false

		if scroll, ok := vimSideScrollKeys[vimKey]; ok {
			if session.zCount > 0 && scroll.columns > 0 {
				scroll.columns = session.zCount
			}

			h.vimScroll(scroll)

			return true
		}

		for _, key := range vimCountKeys(session.zCount) {
			if !h.feedVim(key) {
				return true
			}
		}

		return !h.feedVim("z")
	}

	count, countOnly := vimPendingCount(machine)
	if vimKey != "z" || (!machine.Idle() && !countOnly) {
		return false
	}

	session.zCount = count

	if countOnly && !h.feedVim("<Esc>") {
		return true
	}

	session.zPrefix = true

	h.redrawFrame(h.vimFrame(), "update vim overlay")

	return true
}

// feedVim feeds key to the machine. A panic inside it would take the daemon
// down with the keyboard held, so it is recovered, logged and the mode left.
// It reports whether the session is still open.
func (h *handlerState) feedVim(key vim.Key) bool {
	failed := false

	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				h.logger.Error("Vim machine failed, leaving vim mode",
					zap.String("panic", fmt.Sprint(recovered)))

				failed = true
			}
		}()

		// The error can quote the search pattern, which is never logged. The
		// machine shows it as its message. A refusal from vim mode itself,
		// such as a change too big to make, is a domain error the machine
		// does not show, so the status line says it.
		err := h.vim.machine.Feed(key)
		if err != nil {
			h.logger.Debug("Vim key failed")

			if _, refused := errors.AsType[*derrors.Error](err); refused &&
				h.vim.machine.Message() == "" {
				h.vim.notice = derrors.Message(err)
			}
		}
	}()

	if failed {
		h.exitMode()

		return false
	}

	return true
}

// handleVimIdleKey runs the keys vim mode owns between commands, and reports
// whether key was one.
func (h *handlerState) handleVimIdleKey(key vim.Key) bool {
	switch key {
	case "<Esc>":
		h.exitMode()
	case vimReturn:
		h.vimSelectAtCursor("")
	case "<C-l>":
		h.startVim(h.vimSessionActivation())
	case "s":
		h.vim.jump = &vimJump{}
		h.redrawFrame(h.vimFrame(), "update vim overlay")
	default:
		scroll, ok := vimScrollKeys[key]
		if !ok {
			return false
		}

		h.vimScroll(scroll)
	}

	return true
}

// vimCountTooLong reports a digit that would make the count being typed
// longer than vimMaxCountDigits.
func vimCountTooLong(machine *vim.Machine, key vim.Key) bool {
	if len(key) != 1 || key[0] < '0' || key[0] > '9' {
		return false
	}

	shown := machine.Shown()
	digits := len(shown) - len(strings.TrimRightFunc(shown, unicode.IsDigit))

	return digits >= vimMaxCountDigits
}

// handleVimTypingKey sends a key to the app while typing, until Escape,
// which goes back to normal mode and reads the screen the typing changed.
func (h *handlerState) handleVimTypingKey(key string) {
	session := h.vim

	if configpkg.NormalizeKeyForComparison(key) == configpkg.KeyNameEscape {
		if session.eraseOnEscape {
			h.vimFeed(vimKeyErase)
		}

		session.typing, session.eraseOnEscape = false, false
		session.lastInsert = &vimInsert{command: session.insertCommand, typed: session.typedKeys}

		if !h.feedVim("<Esc>") {
			return
		}

		session.edits, session.edited = nil, false
		h.rereadVim(session.typedAt)

		return
	}

	session.eraseOnEscape = false
	session.typedKeys = append(session.typedKeys, key)

	h.vimFeed(key)
}

// vimStartTyping hands the keyboard to the app at caret, clicking there
// first when the app's caret is not already in place.
func (h *handlerState) vimStartTyping(caret image.Point, click bool) {
	session := h.vim

	if click {
		h.vimClick(caret)
	}

	session.typing = true
	session.typedAt = caret
	session.typedKeys = nil

	h.redrawFrame(h.vimFrame(), "update vim overlay")
}

// vimJump is an s jump. It keeps the text typed so far, its matches nearest
// the cursor first, and each match's label. A match past the last label has
// an empty one.
type vimJump struct {
	query   string
	matches [][2]int
	labels  []string
}

// handleVimJumpKey reads an s jump's keys. A label jumps to its match,
// Return to the nearest, Backspace drops the last character typed and
// Escape gives up. Any other character adds to the text to find.
func (h *handlerState) handleVimJumpKey(key vim.Key) {
	session := h.vim
	jump := session.jump

	switch _, size := utf8.DecodeRuneInString(string(key)); {
	case key == "<Esc>":
		session.jump = nil
	case key == vimReturn:
		if len(jump.matches) > 0 {
			h.vimJumpTo(jump.matches[0][0])

			return
		}

		session.jump = nil
	case key == "<BS>":
		if jump.query == "" {
			session.jump = nil

			break
		}

		_, last := utf8.DecodeLastRuneInString(jump.query)
		jump.query = jump.query[:len(jump.query)-last]

		h.findVimJumpTargets()
	case size != len(key):
		return
	default:
		if index := slices.Index(jump.labels, string(key)); index >= 0 {
			h.vimJumpTo(jump.matches[index][0])

			return
		}

		jump.query += string(key)

		h.findVimJumpTargets()
	}

	h.redrawFrame(h.vimFrame(), "update vim overlay")
}

// vimJumpLabelChars are the labels a jump's matches get, nearest first.
const vimJumpLabelChars = "asdfghjklqwertyuiopzxcvbnm"

// findVimJumpTargets finds where the text typed matches. Like search, it
// ignores case unless the text has a capital. It labels the matches nearest
// the cursor first, and never with the character after a match, so typing
// more of the text never jumps.
func (h *handlerState) findVimJumpTargets() {
	session := h.vim
	jump := session.jump
	jump.matches, jump.labels = nil, nil

	if jump.query == "" {
		return
	}

	query, haystack := jump.query, session.host.Screen().String()
	if strings.ToLower(query) == query {
		haystack = strings.ToLower(haystack)
	}

	next := map[rune]bool{}

	for from := 0; ; {
		found := strings.Index(haystack[from:], query)
		if found < 0 {
			break
		}

		start := from + found
		jump.matches = append(jump.matches, [2]int{start, start + len(query)})

		if r, _ := utf8.DecodeRuneInString(haystack[start+len(query):]); r != utf8.RuneError {
			next[unicode.ToLower(r)] = true
		}

		from = start + 1
	}

	cursor := session.host.Cursor()
	distance := func(match [2]int) int { return max(match[0]-cursor, cursor-match[0]) }
	slices.SortStableFunc(jump.matches, func(a, b [2]int) int {
		return cmp.Compare(distance(a), distance(b))
	})

	free := []rune(vimJumpLabelChars)
	jump.labels = make([]string, len(jump.matches))

	for index := range jump.matches {
		for len(free) > 0 && next[free[0]] {
			free = free[1:]
		}

		if len(free) == 0 {
			break
		}

		jump.labels[index] = string(free[0])
		free = free[1:]
	}
}

// vimJumpLabelBox is where a jump match's label goes: over the character
// after the match, or just past the line's end.
func vimJumpLabelBox(text *screentext.Text, match [2]int) (image.Rectangle, bool) {
	if text.Slice(match[1], min(match[1]+1, text.Len())) != "\n" {
		if box, ok := text.Box(match[1]); ok {
			return box, true
		}
	}

	last, ok := text.Box(match[1] - 1)

	return last.Add(image.Pt(last.Dx(), 0)), ok
}

// vimJumpTo puts the cursor on target and ends the jump.
func (h *handlerState) vimJumpTo(target int) {
	session := h.vim
	session.jump = nil
	session.host.SetCursor(target)

	h.followVimCursor()
	h.redrawFrame(h.vimFrame(), "update vim overlay")
}

// vimScrollKey is how a scroll key scrolls the app: which way, how far as a
// share of the region read, and whether the cursor keeps its place on screen
// or moves with the text.
type vimScrollKey struct {
	direction services.ScrollDirection
	// lines scrolls this many line heights, and columns this many
	// characters sideways, or a fraction of the region when zero.
	lines    int
	columns  int
	fraction float64
	// stays keeps the cursor at the same point on screen, as Ctrl+D does.
	stays bool
}

// vimScrollKeys are the keys that scroll the app rather than the text, since
// the whole screen is already in view.
var vimScrollKeys = map[vim.Key]vimScrollKey{
	"<C-e>": {direction: services.ScrollDirectionDown, lines: vimScrollLines},
	"<C-y>": {direction: services.ScrollDirectionUp, lines: vimScrollLines},
	"<C-d>": {direction: services.ScrollDirectionDown, fraction: vimScrollHalfPage, stays: true},
	"<C-u>": {direction: services.ScrollDirectionUp, fraction: vimScrollHalfPage, stays: true},
	"<C-f>": {direction: services.ScrollDirectionDown, fraction: vimScrollPage, stays: true},
	"<C-b>": {direction: services.ScrollDirectionUp, fraction: vimScrollPage, stays: true},
}

// vimSideScrollColumns is how far zh and zl scroll without a count. One
// character, vim's own default, barely moves a screen.
const vimSideScrollColumns = 8

// vimSideScrollKeys are the second keys after z that scroll the app sideways:
// h and l by characters, and H and L by half the region read.
var vimSideScrollKeys = map[vim.Key]vimScrollKey{
	"h": {direction: services.ScrollDirectionLeft, columns: vimSideScrollColumns},
	"l": {direction: services.ScrollDirectionRight, columns: vimSideScrollColumns},
	"H": {direction: services.ScrollDirectionLeft, fraction: vimScrollHalfPage, stays: true},
	"L": {direction: services.ScrollDirectionRight, fraction: vimScrollHalfPage, stays: true},
}

// vimScrollAtEdge scrolls the app when j or k tried to leave the text read,
// keeps the cursor on the same text, and moves it the line it asked for.
// It reports whether it scrolled.
func (h *handlerState) vimScrollAtEdge(key vim.Key, lineBefore int) bool {
	session := h.vim
	text := session.host.Screen()
	line := text.LineOf(session.host.Cursor())

	var scroll vimScrollKey

	switch {
	case (key == "j" || key == "<Down>") && line == lineBefore && line == text.Lines()-1:
		scroll = vimScrollKey{direction: services.ScrollDirectionDown, fraction: vimScrollEdge}
	case (key == "k" || key == "<Up>") && line == lineBefore && line == 0:
		scroll = vimScrollKey{direction: services.ScrollDirectionUp, fraction: vimScrollEdge}
	default:
		return false
	}

	h.vimScroll(scroll)

	if h.vim != nil && h.feedVim(key) {
		h.followVimCursor()
		h.redrawFrame(h.vimFrame(), "update vim overlay")
	}

	return true
}
