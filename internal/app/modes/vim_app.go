package modes

import (
	"context"
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
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/screentext"
)

const (
	// vimClipboardTimeout bounds a yank's clipboard write, which runs off h.mu.
	vimClipboardTimeout = 2 * time.Second
	// vimScrollSettle is the shortest time between two scrolls, so a held
	// scroll key scrolls no faster than the app keeps up. It bounds the key,
	// not the read, which waits for the screen to stop moving instead.
	vimScrollSettle = 150 * time.Millisecond
	// vimClickSettle is how long the app gets to take a click before keys
	// that act from its caret.
	vimClickSettle = 60 * time.Millisecond
	// vimFlashDuration is how long a yank's flash stays on screen.
	vimFlashDuration = 250 * time.Millisecond
	// vimPastLineEnd is how far right of a line's last character a click
	// lands to put the caret at the line's end.
	vimPastLineEnd = 2
	// vimQuarter divides a size into quarters.
	vimQuarter = 4
	// vimLineSpacing over vimHalf is a line's pitch as a share of its glyphs'
	// height.
	vimLineSpacing = 3
)

// vimLandLimit is how long vimAwaitLine waits for the screen to show a
// change, and vimLandPoll how long it waits between reads. vimLandRows is
// how many lines below the changed one it reads, besides those its new text
// needs, for the text to wrap onto.
const (
	vimLandLimit = time.Second
	vimLandPoll  = 20 * time.Millisecond
	vimLandRows  = 2
)

// The chords vim mode sends the app for u and Ctrl+R, and to copy and paste.
const (
	vimKeyUndo  = "Primary+z"
	vimKeyRedo  = "Primary+Shift+z"
	vimKeyPaste = "Primary+v"
	vimKeyErase = "Delete"
	// vimKeyExtend grows the app's selection by one character, and
	// vimKeyExtendBack grows it backwards.
	vimKeyExtend     = "Shift+Right"
	vimKeyExtendBack = "Shift+Left"
	// vimKeyLeft moves the app's caret back one character.
	vimKeyLeft = "Left"
)

// vimCaretIn is where a click puts the app's caret before the character at
// pos in text: halfway between its middle and the middle of the character
// before it, so a box read a little off on both sides still lands nearest
// that boundary. At the start of a line it is a quarter into the first
// character, and at the end just right of the last.
func vimCaretIn(text *screentext.Text, pos int) image.Point {
	box, ok := text.Box(pos)
	atEnd := !ok || text.Slice(pos, pos+1) == "\n"
	startsLine := pos == 0 || text.Slice(pos-1, pos) == "\n"

	switch {
	case atEnd:
		prev, _ := text.Box(max(min(pos, text.Len())-1, 0))

		return image.Pt(prev.Max.X+vimPastLineEnd, (prev.Min.Y+prev.Max.Y)/vimHalf)
	case startsLine:
		return image.Pt(box.Min.X+box.Dx()/vimQuarter, (box.Min.Y+box.Max.Y)/vimHalf)
	default:
		prev, _ := text.Box(pos - 1)

		// The mean of the four edges is halfway between the two middles.
		return image.Pt(
			(prev.Min.X+prev.Max.X+box.Min.X+box.Max.X)/vimQuarter,
			(box.Min.Y+box.Max.Y)/vimHalf,
		)
	}
}

// vimSelectIn selects [start, end) of text in the app from the keyboard: a
// click puts the caret before start, and one Shift+Right per character the
// app has extends the selection. Keys land exactly, where a Shift+click is
// taken as a plain click by some apps.
//
// The click has to land on text the app has. A range that starts in a line's
// padding clicks the line's first character. A range that starts at a line
// break, as dd on the last line does, clicks its end and selects backwards.
// The line above can be outside the field, such as the title over a search
// bar.
func (h *handlerState) vimSelectIn(text *screentext.Text, start, end int) {
	extend, from := vimKeyExtend, text.AppStart(start)
	if text.Slice(start, min(start+1, text.Len())) == "\n" {
		extend, from = vimKeyExtendBack, end
	}

	h.vimClick(vimCaretIn(text, from))
	h.vimFeed(slices.Repeat([]string{extend}, text.AppChars(start, end))...)
}

// vimAwaitLine waits until the screen shows the line edits are on with the
// edits made. It reads the band of the screen from that line down, since a
// longer line may wrap. An app can take a click ahead of keys sent before
// it, as a browser does, so a click on the next line any sooner would move
// the caret away from keys still on their way. It reports false when the
// screen does not show the change within vimLandLimit.
func (h *handlerState) vimAwaitLine(before *screentext.Text, edits []vim.Edit) bool {
	line := before.LineOf(edits[0].Start)
	start, end := before.LineStart(line), before.Len()

	if line+1 < before.Lines() {
		end = before.LineStart(line+1) - 1
	}

	var changed strings.Builder

	from := start
	for _, edit := range edits {
		changed.WriteString(before.Slice(from, edit.Start))
		changed.WriteString(edit.Text)
		from = edit.End
	}

	changed.WriteString(before.Slice(max(from, start), end))

	level, visible := vimLandLevel(before.Slice(start, end), changed.String())
	if !visible {
		return false
	}

	want := vimLandKey(changed.String(), level)

	first, found := lineRect(before, before.AppStart(start), max(end, start+1))
	if !found {
		return false
	}

	region := h.vim.region
	left := first.Min.X

	if index, ok := before.ContainerAt(start); ok {
		left = min(left, before.Containers()[index].Bounds.Min.X)
	}

	pitch := first.Dy() * vimLineSpacing / vimHalf
	rows := vimLandRows + utf8.RuneCountInString(want)*before.CellWidth()/max(region.Max.X-left, 1)
	band := image.Rect(left, first.Min.Y-first.Dy()/vimQuarter, region.Max.X, first.Min.Y+rows*pitch).
		Intersect(region)

	deadline := time.Now().Add(vimLandLimit)

	for {
		read, _, err := h.readVimText(band)
		if err == nil && strings.Contains(vimLandKey(read.String(), level), want) {
			return true
		}

		if time.Now().After(deadline) {
			return false
		}

		time.Sleep(vimLandPoll)
	}
}

// vimLandLevel is the loosest level of vimLandKey that tells line from what
// it changes to, and false when none does, as for a change of spaces only.
func vimLandLevel(line, changed string) (int, bool) {
	for level := range vimLandLevels {
		if vimLandKey(line, level) != vimLandKey(changed, level) {
			return level, true
		}
	}

	return 0, false
}

// vimLandKey is text as vimAwaitLine compares it, which never has spaces,
// since the fast read guesses them from gaps, or other scripts, which it
// cannot read. At level 0 it keeps Latin letters and digits in lower case,
// with a 0 as an o, and without the thin letters the app's caret reads as,
// since the caret sits just past the text typed. Level 1 keeps letters and
// digits as they are, for a change of case or of a thin letter, and level 2
// keeps punctuation too. The stricter levels match less often while the
// caret blinks on, and wait for it to blink off.
func vimLandKey(text string, level int) string {
	return strings.Map(func(char rune) rune {
		if level == 0 {
			switch char = unicode.ToLower(char); {
			case char == '0':
				return 'o'
			case strings.ContainsRune(vimCaretLike, char):
				return -1
			}
		}

		switch {
		case char > unicode.MaxASCII || unicode.IsSpace(char) || !unicode.IsGraphic(char):
			return -1
		case level < vimLandPunctuation && !unicode.IsLetter(char) && !unicode.IsDigit(char):
			return -1
		}

		return char
	}, text)
}

// vimLandLevels is how many levels vimLandKey has, and vimLandPunctuation
// the first that keeps punctuation.
const (
	vimLandLevels      = 3
	vimLandPunctuation = 2
)

// vimCaretLike are the characters a fast read can see the app's caret as.
const vimCaretLike = "il1"

// vimTypeOver types keys over the selected text of edit, or erases it when
// edit has none.
func (h *handlerState) vimTypeOver(edit vim.Edit, keys []string) {
	if edit.Text == "" && edit.End > edit.Start {
		h.vimFeed(vimKeyErase)

		return
	}

	h.vimFeed(keys...)
}

// vimSameRun reports whether start and end are on one line of one
// container, where the app's arrow keys move between them a character at a
// time. Two fields side by side can share a line read off the screen, and
// an arrow key never crosses from one to the other.
func vimSameRun(text *screentext.Text, start, end int) bool {
	first, inFirst := text.ContainerAt(start)
	last, inLast := text.ContainerAt(end)

	return inFirst == inLast && first == last && !strings.Contains(text.Slice(start, end), "\n")
}

// vimClick left clicks p, and gives the app a moment to place its caret
// there, since the keys that follow act from wherever the caret is.
func (h *handlerState) vimClick(p image.Point) {
	err := h.actionService.PerformActionAtPoint(h.ctx, string(action.NameLeftClick), p, 0)
	if err != nil {
		h.logger.Warn("Vim click failed", zap.Error(err))
	}

	time.Sleep(vimClickSettle)
}

// vimDragRange presses at the start of [start, end), moves to its end and
// lets go, so the app under it selects that text itself.
func (h *handlerState) vimDragRange(start, end int) {
	text := h.vim.host.Screen()
	press, release := vimCaretIn(text, start), vimCaretIn(text, max(end, start+1))

	steps := []struct {
		name  action.Name
		point image.Point
	}{
		{action.NameLeftMouseDown, press},
		{action.NameMoveMouse, release},
		{action.NameLeftMouseUp, release},
	}

	for _, step := range steps {
		err := h.actionService.PerformActionAtPoint(h.ctx, string(step.name), step.point, 0)
		if err != nil {
			h.logger.Warn("Vim drag failed", zap.String("step", string(step.name)), zap.Error(err))

			return
		}
	}
}

// vimFeed sends keys to the app. A key that fails stops the rest.
func (h *handlerState) vimFeed(keys ...string) bool {
	if h.keyFeed == nil {
		return false
	}

	for _, key := range keys {
		// A fed key is a keystream, so neither it nor the error is logged.
		err := h.keyFeed.Feed(h.ctx, key)
		if err != nil {
			h.logger.Debug("Failed to send a key to the app")

			return false
		}
	}

	return true
}

// vimApplyEdits makes a command's edits in the app with clicks and keys. A
// deletion selects the text and erases it, a replacement selects it and
// types over it, an insertion clicks into place and types, and a put clicks
// and pastes. They run last to first, so
// each one's place on screen has not moved yet. Then the screen is read
// again, or the keyboard goes to the app when the command left insert mode.
func (h *handlerState) vimApplyEdits(edits []vim.Edit, label string) {
	session := h.vim
	before := session.before
	put := vimIsPut(label)
	anchor := vimCaretIn(before, edits[0].Start)

	// A change leaves its text selected for the first key typed to replace.
	// Erasing it first would let the app's smart delete take a neighboring
	// space with it.
	if len(edits) == 1 && edits[0].Text == "" && edits[0].End > edits[0].Start &&
		session.machine.Mode() == vim.Insert {
		h.vimSelectIn(before, edits[0].Start, edits[0].End)
		session.edits = nil
		session.eraseOnEscape = true

		h.vimStartTyping(anchor, false)

		return
	}

	held, heldErr := h.actionService.CursorPosition(h.ctx)

	// after is the change made last, which is later in the text, and typed
	// how many characters the caret sits past its start. Nil after a paste,
	// whose length the app decides.
	var after *vim.Edit

	typed := 0

	// lineEdits are the changes made since the last click, first to last,
	// all on one line of before, which the screen has to show before a click
	// on another line.
	var lineEdits []vim.Edit

	for index, edit := range slices.Backward(edits) {
		keys, typeable := screentext.FeedKeys(edit.Text)
		if !put && !typeable {
			h.logger.Debug("Vim edit has text that cannot be typed")

			break
		}

		chained := after != nil && vimSameRun(before, edit.Start, after.Start)

		if !chained && lineEdits != nil {
			if !h.vimAwaitLine(before, lineEdits) {
				session.notice = fmt.Sprintf(
					"stopped with %d left, the app did not show the last change", index+1)

				break
			}

			lineEdits = nil
		}

		switch {
		case put:
			h.vimClick(vimCaretIn(before, edit.Start))
			h.vimFeed(vimKeyPaste)
		case chained:
			// The caret is just past what the change after this one typed, on
			// the same run of text, so arrow keys reach this one without a
			// click, which would move the pointer and wait for the app.
			h.vimFeed(
				slices.Repeat(
					[]string{vimKeyLeft},
					typed+before.AppChars(edit.End, after.Start),
				)...)
			h.vimFeed(
				slices.Repeat([]string{vimKeyExtendBack}, before.AppChars(edit.Start, edit.End))...)
			h.vimTypeOver(edit, keys)
		case edit.End > edit.Start:
			h.vimSelectIn(before, edit.Start, edit.End)
			h.vimTypeOver(edit, keys)
		default:
			h.vimClick(vimCaretIn(before, edit.Start))
			h.vimFeed(keys...)
		}

		lineEdits = append([]vim.Edit{edit}, lineEdits...)

		after, typed = nil, len(keys)
		if !put {
			after = &edit
		}
	}

	session.edits = nil

	// With the pointer held, the clicks that placed the caret leave it where
	// the user had it.
	if !session.follow && heldErr == nil {
		_ = h.actionService.MoveCursorToPoint(h.ctx, held)
	}

	if session.machine.Mode() == vim.Insert {
		h.vimStartTyping(anchor, false)

		return
	}

	session.lastInsert = nil

	h.rereadVim(anchor)
}

// vimIsPut reports whether a command's keys are a put, p or P after any
// register and count.
func vimIsPut(label string) bool {
	if len(label) >= 2 && label[0] == '"' {
		label = label[2:]
	}

	label = strings.TrimLeft(label, "0123456789")

	return label == "p" || label == "P" || label == "gp" || label == "gP"
}

// vimYank copies the text a yank took, as it was read, without touching the
// app, and flashes where it was taken from so the copy shows. Icons in it are
// left out, since the app has no text for them.
func (h *handlerState) vimYank(read string) {
	session := h.vim
	text := session.host.Screen()

	spans, _ := session.machine.Flash()
	session.flash = nil

	if len(spans) > 0 {
		parts := make([]string, 0, len(spans))
		for _, span := range spans {
			parts = append(parts, text.AppText(span[0], span[1]))
		}

		read = strings.Join(parts, "\n")
	}

	for _, span := range spans {
		for _, seg := range lineSegments(text, span[0], span[1]) {
			if r, ok := lineRect(text, seg[0], seg[1]); ok {
				session.flash = append(session.flash, r)
			}
		}
	}

	session.notice = vimCopiedNotice(read)

	outer := h.outer
	time.AfterFunc(vimFlashDuration, func() { outer.clearVimFlash(session) })

	clipboard, logger, parent := h.clipboard, h.logger, h.ctx
	if clipboard == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(parent, vimClipboardTimeout)
		defer cancel()

		err := clipboard.WriteText(ctx, read)
		if err != nil {
			logger.Warn("Failed to copy the yank to the clipboard", zap.Error(err))
		}
	}()
}

// vimCopiedNotice says how much a yank copied.
func vimCopiedNotice(read string) string {
	if lines := strings.Count(strings.TrimSuffix(read, "\n"), "\n") + 1; lines > 1 {
		return "copied " + strconv.Itoa(lines) + " lines"
	}

	return "copied " + strconv.Itoa(utf8.RuneCountInString(read)) + " characters"
}

// clearVimFlash takes a yank's flash off the screen, for the session that
// made it.
func (h *Handler) clearVimFlash(session *vimSession) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.vim != session || session.flash == nil {
		return
	}

	session.flash = nil

	h.redrawFrame(h.vimFrame(), "update vim overlay")
}

// vimUndo sends the app its own undo or redo and reads the screen again.
func (h *handlerState) vimUndo(chord string) {
	at := h.vimCursorPoint()

	if h.vimFeed(chord) {
		h.rereadVim(at)
	}
}

// vimScroll scrolls the app under the cursor and leaves the text read stale.
// Scroll events go to the window under the pointer, so a session holding the
// pointer still moves it there and back. The screen is read off h.mu once
// the app has moved it and it stops moving, and the next scroll cancels that
// read, so a run of scrolls reads only the screen it ends on.
func (h *handlerState) vimScroll(scroll vimScrollKey) {
	session := h.vim

	// A held key repeats faster than the app scrolls. A scroll key that
	// arrives within vimScrollSettle of the last scroll is dropped, so a
	// held key scrolls at a pace the app keeps up with.
	if time.Since(session.scrolled) < vimScrollSettle {
		return
	}

	session.scrolled = time.Now()
	session.afterScroll = ""

	point := h.vimCursorPoint()
	text := session.host.Screen()

	sideways := scroll.direction == services.ScrollDirectionLeft ||
		scroll.direction == services.ScrollDirectionRight

	step := int(float64(session.shown.Dy()) * scroll.fraction)

	switch {
	case scroll.pixels > 0:
		step = scroll.pixels
	case sideways && scroll.columns > 0:
		step = scroll.columns * text.CellWidth()
	case sideways:
		step = int(float64(session.shown.Dx()) * scroll.fraction)
	case scroll.lines > 0:
		box, _ := text.Box(session.host.Cursor())
		step = scroll.lines * max(box.Dy()*vimLineSpacing/vimHalf, 1)
	}

	held, heldErr := h.actionService.CursorPosition(h.ctx)
	if !session.follow {
		_ = h.actionService.MoveCursorToPoint(h.ctx, point)
	}

	err := h.scrollService.Scroll(h.ctx, scroll.direction, services.ScrollAmountChar, step, 0)
	if err != nil {
		h.logger.Warn("Vim scroll failed", zap.Error(err))
	}

	if !session.follow && heldErr == nil {
		_ = h.actionService.MoveCursorToPoint(h.ctx, held)
	}

	near := point
	if session.stale {
		near = session.staleNear
	}

	if !scroll.stays {
		switch scroll.direction {
		case services.ScrollDirectionDown:
			near.Y -= step
		case services.ScrollDirectionUp:
			near.Y += step
		case services.ScrollDirectionRight:
			near.X -= step
		case services.ScrollDirectionLeft:
			near.X += step
		}
	}

	// The text read is for the screen before the scroll, so applyVimRefine
	// drops an accurate read of it when it lands.
	session.stale, session.staleNear = true, near
	session.generation++
	session.refined = nil
	session.scrollSeq++

	// The frame is drawn before the read starts, so its first capture already
	// has the moved frame to leave out.
	h.cancelYoungRefine()
	h.redrawFrame(h.vimFrame(), "update vim overlay")
	h.readVimAfterScroll()
}

// vimMouseAtCursor runs a mouse action, such as a click, at the character
// under the cursor as many times as times, at most vimMaxClicks, so two
// clicks are a double click. The mode stays open and reads the screen again,
// since the action most likely changed it. A press only redraws. The screen
// changes on its release, and reading it while the button is down shows the
// overlay again, which ends the app's drag.
func (h *handlerState) vimMouseAtCursor(name action.Name, times int) {
	clicks := slices.Repeat([]string{string(name)}, min(max(times, 1), vimMaxClicks))
	chain := strings.Join(clicks, ",")

	after := func() { h.startVim(h.vimSessionActivation()) }
	if name == action.NameLeftMouseDown || name == action.NameRightMouseDown ||
		name == action.NameMiddleMouseDown {
		after = func() { h.redrawFrame(h.vimFrame(), "update vim overlay") }
	}

	h.moveCursorAndHandleAction(h.vimCursorPoint(), &chain, nil, true, after)
}
