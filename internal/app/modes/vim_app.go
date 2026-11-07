package modes

import (
	"context"
	"image"
	"slices"
	"strconv"
	"strings"
	"time"
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
	// vimScrollSettle is how long a scroll gets to finish moving, smooth
	// scrolling included, before the screen is read again.
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

	for _, edit := range slices.Backward(edits) {
		keys, typeable := screentext.FeedKeys(edit.Text)
		if !put && !typeable {
			h.logger.Debug("Vim edit has text that cannot be typed")

			break
		}

		switch {
		case put:
			h.vimClick(vimCaretIn(before, edit.Start))
			h.vimFeed(vimKeyPaste)
		case edit.End > edit.Start:
			h.vimSelectIn(before, edit.Start, edit.End)

			if edit.Text == "" {
				h.vimFeed(vimKeyErase)
			} else {
				h.vimFeed(keys...)
			}
		default:
			h.vimClick(vimCaretIn(before, edit.Start))
			h.vimFeed(keys...)
		}
	}

	session.edits = nil

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

// vimScroll scrolls the app under the cursor and reads the screen again.
// Scroll events go to the window under the pointer, so a session holding the
// pointer still moves it there and back.
func (h *handlerState) vimScroll(scroll vimScrollKey) {
	session := h.vim

	// A held key repeats faster than a scroll and its re-read finish. Each
	// repeat waits on h.mu, so the queued ones would keep the app scrolling,
	// and Escape waiting, long after the key is let go. A scroll key that
	// arrives within vimScrollSettle of the last scroll ending is dropped,
	// which empties the queue at once.
	if time.Since(session.scrolled) < vimScrollSettle {
		return
	}

	defer func() { session.scrolled = time.Now() }()

	point := h.vimCursorPoint()
	text := session.host.Screen()

	sideways := scroll.direction == services.ScrollDirectionLeft ||
		scroll.direction == services.ScrollDirectionRight

	step := int(float64(session.region.Dy()) * scroll.fraction)

	switch {
	case sideways && scroll.columns > 0:
		step = scroll.columns * text.CellWidth()
	case sideways:
		step = int(float64(session.region.Dx()) * scroll.fraction)
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

	time.Sleep(vimScrollSettle)

	if !scroll.stays {
		switch scroll.direction {
		case services.ScrollDirectionDown:
			point.Y -= step
		case services.ScrollDirectionUp:
			point.Y += step
		case services.ScrollDirectionRight:
			point.X -= step
		case services.ScrollDirectionLeft:
			point.X += step
		}
	}

	h.rereadVim(point)
}

// vimSelectAtCursor runs a click at the character under the cursor.
func (h *handlerState) vimSelectAtCursor(operator string) {
	h.vimSelect(h.vimCursorPoint(), operator)
}

// vimSelect runs the click an operator names at point, or with no operator the
// activation's --action, or a left click. It ends the mode as a hint
// selection does, or reads the screen again with --repeat.
func (h *handlerState) vimSelect(point image.Point, operator string) {
	session := h.vim

	actionName := string(action.NameLeftClick)
	if named, ok := vimOperatorActions[operator]; ok {
		actionName = named
	} else if session.action != nil {
		actionName = *session.action
	}

	h.moveCursorAndHandleAction(point, &actionName, session.modifier, session.repeat, func() {
		h.startVim(h.vimSessionActivation())
	})
}
