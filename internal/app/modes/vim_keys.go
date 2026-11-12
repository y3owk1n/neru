package modes

import (
	"cmp"
	"errors"
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/y3owk1n/fude/vim"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/screentext"
)

// vimReturn is Return in vim's notation, and vimEscape Escape.
const (
	vimReturn vim.Key = "<CR>"
	vimEscape vim.Key = "<Esc>"
	// vimTab and vimShiftTab cycle through the ex commands on the command
	// line.
	vimTab      vim.Key = "\t"
	vimShiftTab vim.Key = "<S-Tab>"
)

// vimControlKeys are the Ctrl chords vim's own commands read, held back from
// the focused app while a session is open so they reach the machine, or while
// typing, so they reach the app through the key feed. The chords the keymap
// maps are held back with them.
const vimControlKeys = "abdefgilnoprtuvwxy["

// vimMapTimeout is how long the machine holds keys that start a longer
// mapping before it runs them as typed, vim's own timeoutlen.
const vimMapTimeout = time.Second

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

	chords := h.vim.keymap.Chords()
	for _, key := range vimControlKeys {
		chords = append(chords, "Ctrl+"+strings.ToUpper(string(key)))
	}

	slices.Sort(chords)

	return slices.Compact(chords)
}

// handleVimKey runs one key. With nothing pending, Escape leaves the mode.
// Everything else goes to the machine, which runs vim mode's own commands
// through the keymap like its own, and what it asks of the app is done once
// it returns.
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

	if session.stale {
		if scroll, ok := h.vimEdgeScroll(vimKey); ok {
			h.vimScroll(scroll)

			session.afterScroll = vimKey

			return
		}

		// Every key but another scroll, or Escape, acts on the text, so it
		// first reads the screen the scrolls left.
		if vimKey != vimEscape && !h.vimContinuesScroll(vimKey) {
			h.readScrolledVim()

			if h.vim != session {
				return
			}
		}
	}

	if session.jump != nil {
		h.handleVimJumpKey(vimKey)

		return
	}

	h.handleVimMachineKey(vimKey)
}

// handleVimMachineKey runs a key in vim's notation: the keys vim mode keeps
// from the machine, or the machine's, and what the machine asks of the app
// once it returns.
func (h *handlerState) handleVimMachineKey(vimKey vim.Key) {
	session := h.vim
	machine := session.machine

	if vimKey == vimTab || vimKey == vimShiftTab {
		if h.vimCompleteEx(vimKey == vimShiftTab) {
			return
		}
	}

	session.completion = nil

	if vimKey == "." && session.lastInsert != nil && !session.repeating &&
		!h.vimMapsKey(vimKey) {
		if count, ok := vimPendingCount(machine); ok || machine.Idle() {
			if ok && !h.feedVim(vimEscape) {
				return
			}

			h.vimRepeatInsert(count)

			return
		}
	}

	if machine.Idle() {
		session.commandKeys = nil

		if vimKey == vimEscape {
			h.exitMode()

			return
		}
	}

	// Escape drops keys held for a longer mapping without running them.
	if vimKey == vimEscape && machine.Waiting() {
		h.dropVimHeld()
		h.redrawFrame(h.vimFrame(), "update vim overlay")

		return
	}

	if vimCountTooLong(machine, vimKey) {
		return
	}

	session.commandKeys = append(session.commandKeys, vimKey)

	h.vimStep(vimKey, func() error { return machine.Feed(vimKey) })
}

// vimStep runs step, which feeds key to the machine, or with no key ends its
// wait for a longer mapping, and then does what the machine asked of the app.
func (h *handlerState) vimStep(key vim.Key, step func() error) {
	session := h.vim
	machine := session.machine

	line := session.host.Screen().LineOf(session.host.Cursor())

	// The keymap applies only between commands in normal and visual mode,
	// and only then does vimStep track the held keys and the j or k a key
	// comes to.
	var held []vim.Key

	edge := vim.Key("")

	if key != "" && (machine.Waiting() || vimBareKey(machine)) {
		held = session.keymap.Held(machine.Mode(), session.held, key)
		edge = h.vimEdgeKey(key)
	}

	session.edited, session.edits, session.yank, session.notice = false, nil, "", ""

	if !h.runVim(step) {
		return
	}

	session.held = held

	h.vimAwaitMapping()

	if machine.Mode() == vim.Insert {
		session.insertCommand = slices.Clone(session.commandKeys)
	}

	acts := session.acts
	session.acts = nil

	if len(session.edits) > 0 {
		h.vimApplyEdits(session.edits, session.label)

		if len(acts) == 0 || h.vim != session || session.typing {
			return
		}
	}

	if len(acts) > 0 {
		for _, act := range acts {
			act()

			if h.vim != session {
				return
			}
		}

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

	if edge != "" && h.vimScrollAtEdge(key, edge, line) {
		return
	}

	h.vimCursorMoved()
}

// vimAwaitMapping gives keys the machine holds for a longer mapping
// vimMapTimeout for the rest of it, then runs them as typed. Each key starts
// the wait over, and one that ends the hold ends it.
func (h *handlerState) vimAwaitMapping() {
	session := h.vim
	session.mapWait++

	if session.mapTimer != nil {
		session.mapTimer.Stop()
		session.mapTimer = nil
	}

	if !session.machine.Waiting() {
		return
	}

	outer, wait := h.outer, session.mapWait
	session.mapTimer = time.AfterFunc(vimMapTimeout, func() { outer.endVimMapWait(session, wait) })
}

// endVimMapWait runs the keys the machine held for a mapping that never came,
// for the session and the wait that started the timer. Text a scroll left
// stale is read first, since the keys may act on it.
func (h *Handler) endVimMapWait(session *vimSession, wait uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.vim != session || session.mapWait != wait || session.jump != nil || session.typing {
		return
	}

	if session.stale {
		h.readScrolledVim()

		if h.vim != session {
			return
		}
	}

	h.vimStep("", session.machine.Timeout)
}

// vimMapsKey reports whether key, typed next, is mapped or starts a mapping.
func (h *handlerState) vimMapsKey(key vim.Key) bool {
	session := h.vim
	keys := append(slices.Clone(session.held), key)

	return session.keymap.Leads(session.machine.Mode(), keys, func([]vim.Key) bool { return true })
}

// vimEdgeKey returns the key that key comes to through the keymap, so a key
// mapped to j or k scrolls at the edge as they do. That is the one key a
// mapping of the held keys and key runs, key itself when no mapping takes
// it, and nothing otherwise.
func (h *handlerState) vimEdgeKey(key vim.Key) vim.Key {
	session := h.vim
	keys := append(slices.Clone(session.held), key)

	if to, ok := session.keymap.Runs(session.machine.Mode(), keys); ok {
		if len(to) == 1 {
			return to[0]
		}

		return ""
	}

	if len(session.held) > 0 {
		return ""
	}

	return key
}

// vimCursorMoved moves the real cursor to the machine's cursor, then swaps in
// an accurate read that is waiting, or redraws the overlay when none is.
func (h *handlerState) vimCursorMoved() {
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

// feedVim feeds key to the machine, and reports whether the session is still
// open. It feeds past vimStep, so it clears the held keys vimStep tracks once
// the machine holds none.
func (h *handlerState) feedVim(key vim.Key) bool {
	session := h.vim
	if !h.runVim(func() error { return session.machine.Feed(key) }) {
		return false
	}

	if !session.machine.Waiting() {
		session.held = nil
	}

	return true
}

// dropVimHeld drops the keys the machine holds for a longer mapping, running
// none of them, and stops the wait for the rest. It returns them.
func (h *handlerState) dropVimHeld() []vim.Key {
	session := h.vim
	held := session.held

	session.machine.Cancel()
	session.held = nil

	h.vimAwaitMapping()

	return held
}

// startVimJump starts a jump. Keys held for a longer mapping when it starts
// were typed after it, so the characters among them begin its text.
func (h *handlerState) startVimJump() {
	session := h.vim
	session.jump = &vimJump{}

	for _, key := range h.dropVimHeld() {
		if utf8.RuneCountInString(string(key)) == 1 {
			session.jump.query += string(key)
		}
	}

	if session.jump.query != "" {
		h.findVimJumpTargets()
	}

	h.redrawFrame(h.vimFrame(), "update vim overlay")
}

// runVim runs step, which hands the machine a key. A panic inside it would
// take the daemon down with the keyboard held, so it is recovered, logged and
// the mode left. It reports whether the session is still open.
func (h *handlerState) runVim(step func() error) bool {
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
		err := step()
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

		// The command that started the typing made this key's edit. Escape
		// can make one of its own, as when it drops an autoindent the machine
		// believes nothing was typed after, and a refused edit would leave
		// the machine in insert mode. Its edit is dropped below, since the
		// screen is read again instead.
		session.edits, session.edited = nil, false

		if !h.feedVim(vimEscape) {
			return
		}

		session.edits, session.edited = nil, false

		generation := session.generation
		h.rereadVim(session.typedAt)

		if h.vim == session && session.generation != generation {
			session.host.SetCursor(
				vimCursorAfterTyping(session.host.Screen(), session.typedAt, session.typedKeys),
			)
			h.redrawFrame(h.vimFrame(), "update vim overlay")
		}

		return
	}

	session.eraseOnEscape = false
	session.typedKeys = append(session.typedKeys, key)

	h.vimFeed(key)
}

// vimCursorAfterTyping is where vim's cursor goes when typing that began at
// caret ends: on the last character typed, as vim leaves it. When a key other
// than a character, Space or Backspace moved the app's caret in a way vim
// mode cannot follow, it is the first character typed.
func vimCursorAfterTyping(text *screentext.Text, caret image.Point, keys []string) int {
	// caret is between two characters, so the first one typed is just right.
	first := text.Nearest(caret.Add(image.Pt(1, 0)))

	length, known := vimTypedLength(keys)
	if !known || length == 0 {
		return first
	}

	end := first
	for end < text.Len() && text.Slice(end, end+1) != "\n" {
		end++
	}

	chars := screentext.Characters(text.Slice(first, end))

	pos := first
	for _, char := range chars[:min(length, len(chars))-1] {
		pos += len(char)
	}

	return pos
}

// vimTypedLength is how many characters typing keys left after where it
// began, when every key is a character, Space or Backspace. Any other key,
// or a Backspace past where typing began, reports false.
func vimTypedLength(keys []string) (int, bool) {
	length := 0

	for _, key := range keys {
		switch name := configpkg.NormalizeKeyForComparison(key); {
		case name == configpkg.KeyNameSpace:
			length++
		case name == configpkg.KeyNameDelete:
			length--
			if length < 0 {
				return 0, false
			}
		case utf8.RuneCountInString(strings.TrimPrefix(name, "shift+")) == 1:
			length++
		default:
			return 0, false
		}
	}

	return length, true
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
	case key == vimEscape:
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

// vimJumpTo ends the jump and moves the cursor to target with go, the
// machine's own motion to a byte, so the machine remembers the column as it
// does after j. The count skips the cap on a typed count, since a byte offset
// can need more digits.
func (h *handlerState) vimJumpTo(target int) {
	h.vim.jump = nil

	for _, key := range append(vimCountKeys(target+1), vimPlugGo) {
		if !h.feedVim(key) {
			return
		}
	}

	h.vimCursorMoved()
}

// vimScrollKey is how a scroll key scrolls the app: which way, how far as a
// share of the region read, and whether the cursor keeps its place on screen
// or moves with the text.
type vimScrollKey struct {
	direction services.ScrollDirection
	// pixels scrolls this far, lines this many line heights, and columns
	// this many characters sideways, or a fraction of the region when all
	// are zero.
	pixels   int
	lines    int
	columns  int
	fraction float64
	// stays keeps the cursor at the same point on screen, as Ctrl+D does.
	stays bool
}

// vimScrollPlugs are vim mode's commands that scroll the app rather than the
// text, since the whole screen is already in view: by line heights or a share
// of the region read up and down, and by characters or half the region
// sideways.
var vimScrollPlugs = map[string]vimScrollKey{
	screentext.PlugScrollLineDown: {direction: services.ScrollDirectionDown, lines: vimScrollLines},
	screentext.PlugScrollLineUp:   {direction: services.ScrollDirectionUp, lines: vimScrollLines},
	screentext.PlugScrollHalfDown: {
		direction: services.ScrollDirectionDown, fraction: vimScrollHalfPage, stays: true,
	},
	screentext.PlugScrollHalfUp: {
		direction: services.ScrollDirectionUp, fraction: vimScrollHalfPage, stays: true,
	},
	screentext.PlugScrollPageDown: {
		direction: services.ScrollDirectionDown, fraction: vimScrollPage, stays: true,
	},
	screentext.PlugScrollPageUp: {
		direction: services.ScrollDirectionUp, fraction: vimScrollPage, stays: true,
	},
	screentext.PlugScrollLeft: {
		direction: services.ScrollDirectionLeft, columns: vimSideScrollColumns,
	},
	screentext.PlugScrollRight: {
		direction: services.ScrollDirectionRight, columns: vimSideScrollColumns,
	},
	screentext.PlugScrollHalfLeft: {
		direction: services.ScrollDirectionLeft, fraction: vimScrollHalfPage, stays: true,
	},
	screentext.PlugScrollHalfRight: {
		direction: services.ScrollDirectionRight, fraction: vimScrollHalfPage, stays: true,
	},
}

// vimSideScrollColumns is how far the sideways scroll by characters goes
// without a count. One character, vim's own default, barely moves a screen.
const vimSideScrollColumns = 8

// vimScrollPlug returns the scroll a key runs when it is one of
// vimScrollPlugs.
func vimScrollPlug(key vim.Key) (vimScrollKey, bool) {
	name, isPlug := strings.CutPrefix(string(key), "<Plug>(")
	if !isPlug {
		return vimScrollKey{}, false
	}

	scroll, found := vimScrollPlugs[strings.TrimSuffix(name, ")")]

	return scroll, found
}

// vimCenterLine scrolls the app until the cursor's line sits in the middle
// of the region read, and the cursor stays on its text. When the line is
// already there it only redraws, since a count may have moved the cursor.
func (h *handlerState) vimCenterLine() {
	session := h.vim
	region := session.region

	step := 0
	if box, ok := session.host.Screen().Box(session.host.Cursor()); ok {
		step = box.Min.Y - region.Min.Y - (region.Dy()-box.Dy())/vimHalf
	}

	switch {
	case step > 0:
		h.vimScroll(vimScrollKey{direction: services.ScrollDirectionDown, pixels: step})
	case step < 0:
		h.vimScroll(vimScrollKey{direction: services.ScrollDirectionUp, pixels: -step})
	default:
		h.redrawFrame(h.vimFrame(), "update vim overlay")
	}
}

// vimScrollAtEdge scrolls the app when key, which came to edge through the
// keymap, was a j or k that tried to leave the text read, keeps the cursor
// on the same text, and moves it the line it asked for. It reports whether
// it scrolled.
func (h *handlerState) vimScrollAtEdge(key, edge vim.Key, lineBefore int) bool {
	session := h.vim
	text := session.host.Screen()
	line := text.LineOf(session.host.Cursor())

	var scroll vimScrollKey

	switch {
	case (edge == "j" || edge == "<Down>") && line == lineBefore && line == text.Lines()-1:
		scroll = vimScrollKey{direction: services.ScrollDirectionDown, fraction: vimScrollEdge}
	case (edge == "k" || edge == "<Up>") && line == lineBefore && line == 0:
		scroll = vimScrollKey{direction: services.ScrollDirectionUp, fraction: vimScrollEdge}
	default:
		return false
	}

	h.vimScroll(scroll)

	// The cursor moves the line once the screen is read again.
	if session.stale {
		session.afterScroll = key
	}

	return true
}

// vimCompleteEx puts the next ex command that starts with the name typed on
// the command line, or the one before it with back. After the last match it
// puts back the name typed. It takes the key only on the : line while only a
// name is typed, and reports whether it did.
func (h *handlerState) vimCompleteEx(back bool) bool {
	session := h.vim
	machine := session.machine
	completion := session.completion

	if completion == nil {
		typed, ok := strings.CutPrefix(machine.Line(), ":")
		if !ok || machine.Mode() != vim.Cmdline || strings.TrimFunc(typed, unicode.IsLetter) != "" {
			return false
		}

		completion = &vimCompletion{typed: typed}

		for _, name := range vimExNames() {
			if strings.HasPrefix(name, typed) {
				completion.matches = append(completion.matches, name)
			}
		}

		// No command starts with it, so Tab does nothing, as in vim.
		if len(completion.matches) == 0 {
			return true
		}

		completion.at = len(completion.matches)
		session.completion = completion
	}

	slots := len(completion.matches) + 1
	if back {
		completion.at = (completion.at + slots - 1) % slots
	} else {
		completion.at = (completion.at + 1) % slots
	}

	line := completion.typed
	if completion.at < len(completion.matches) {
		line = completion.matches[completion.at]
	}

	machine.SetLine(line)
	h.redrawFrame(h.vimFrame(), "update vim overlay")

	return true
}

// vimBareKey reports whether the next key is a command on its own, with at
// most a count before it, in normal or visual mode. A j or k typed on the
// command line, or after f, is text rather than a move, so it never scrolls.
func vimBareKey(machine *vim.Machine) bool {
	mode := machine.Mode()

	return (mode == vim.Normal || mode == vim.Visual) && !machine.Pending() &&
		strings.TrimFunc(machine.Shown(), unicode.IsDigit) == ""
}

// vimEdgeScroll is the scroll a j or k makes while the text is stale from
// that same key scrolling at an edge, so holding it keeps scrolling without
// a read per scroll.
func (h *handlerState) vimEdgeScroll(key vim.Key) (vimScrollKey, bool) {
	if key != h.vim.afterScroll {
		return vimScrollKey{}, false
	}

	switch h.vimEdgeKey(key) {
	case "j", "<Down>":
		return vimScrollKey{direction: services.ScrollDirectionDown, fraction: vimScrollEdge}, true
	case "k", "<Up>":
		return vimScrollKey{direction: services.ScrollDirectionUp, fraction: vimScrollEdge}, true
	default:
		return vimScrollKey{}, false
	}
}

// vimContinuesScroll reports a key that scrolls again without reading the
// text: one that, after the keys the machine holds, runs a scroll of the app
// or starts a mapping that runs one, as the z of zl does.
func (h *handlerState) vimContinuesScroll(key vim.Key) bool {
	session := h.vim
	machine := session.machine

	if !machine.Waiting() && !vimBareKey(machine) {
		return false
	}

	keys := append(slices.Clone(session.held), key)

	return session.keymap.Leads(machine.Mode(), keys, func(to []vim.Key) bool {
		if len(to) != 1 {
			return false
		}

		_, ok := vimScrollPlug(to[0])

		return ok
	})
}
