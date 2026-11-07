package modes

import (
	"context"
	"image"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/y3owk1n/fude/vim"
	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/screentext"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	// vimRefineTimeout bounds the accurate read that follows an activation.
	// One that meets a script for the first time compiles its model, which
	// takes seconds.
	vimRefineTimeout = 30 * time.Second
	// vimSettleDelay is how long the app gets to redraw after vim mode acted
	// on it, before the screen is read again.
	vimSettleDelay = 80 * time.Millisecond
	// vimHalf halves a size to find a middle.
	vimHalf = 2
)

// vimSlowRead is how long an accurate read runs before the overlay says it is
// slow. A warm one takes well under a second. A variable so tests need not
// wait it out.
var vimSlowRead = 1500 * time.Millisecond

// vimSession is an open vim mode: the text read off the screen, the machine
// moving a cursor over it, and what the activation asked for.
type vimSession struct {
	machine *vim.Machine
	host    *screentext.Host
	region  image.Rectangle // what was read, global

	scope    string
	follow   bool
	action   *string
	modifier *string
	repeat   bool
	onExit   []string

	// What a key asked of the app, done once Feed returns, since the machine
	// is still in the middle of the key when it calls back.
	act   func()
	edits []vim.Edit
	label string
	// before is the text the edits' offsets are in, as it was on screen.
	before *screentext.Text
	yank   string
	// edited reports that this key already handed over edits. A second batch
	// in the same key, from a count or a macro, would land on offsets the
	// first one moved, so it is refused.
	edited bool

	// typing is insert mode: keys go to the app, until Escape.
	typing bool
	// typedAt is where typing began, which is where the cursor goes after.
	typedAt image.Point
	// eraseOnEscape is a change whose text is still selected in the app.
	// The first key typed replaces it, and Escape before any erases it.
	eraseOnEscape bool

	// generation counts reads, so a late accurate read of an older screen is
	// dropped. reading is true until the accurate read lands, and refined
	// holds it while the machine is mid-command.
	generation uint64
	reading    bool
	refined    *screentext.Text
	// refining is an accurate read in flight. One runs at a time, and a read
	// asked for meanwhile waits for it. slowRead is set once it has taken
	// longer than vimSlowRead, for the overlay to say why.
	refining bool
	slowRead bool

	// scrolled is when the last scroll of the app finished.
	scrolled time.Time

	jump *vimJump

	// commandKeys are the keys of the command being typed, and insertCommand
	// those of the one that started typing into the app. typedKeys are the
	// keys typed into the app, and lastInsert the last command and typing
	// together, which . repeats, since the machine never saw the typing.
	commandKeys   []vim.Key
	insertCommand []vim.Key
	typedKeys     []string
	lastInsert    *vimInsert
	repeating     bool

	// zPrefix is a z waiting for its second key, with the count typed before
	// it, for the sideways scroll keys.
	zPrefix bool
	zCount  int

	// flash is where the last yank took its text, shown briefly, and notice
	// what the status bar says about it until the next key.
	flash  []image.Rectangle
	notice string
}

// startVim reads the screen and opens vim mode over it. Activated again while
// open, it reads the screen again and keeps the machine, so registers,
// macros, marks and the last search carry over, with the cursor put back
// nearest where it was.
func (h *handlerState) startVim(activation modecmd.Activation) {
	isRefresh := h.appState.CurrentMode() == domain.ModeVim && h.vim != nil
	start := time.Now()

	_, activated := h.activateModeBase(
		domain.ModeNameVim,
		h.config.Vim.Enabled,
		action.TypeMoveMouse,
		"",
	)
	if !activated {
		h.abandonVim(isRefresh)

		return
	}

	if isRefresh {
		h.stopIndicatorPolling()
	} else {
		// Coming from another mode, the keyboard is handed over rather than
		// given back. Exit now, release at return if nothing is entered.
		defer h.exitModeForTransition()()
	}

	// A platform that cannot read lines refuses before the permission is
	// asked for, or the user grants screen sharing for a mode that can never
	// open. An empty region answers without capturing anything.
	_, probeErr := h.readVimText(image.Rectangle{}, ports.TextFast)
	if derrors.IsNotSupported(probeErr) {
		h.logger.Warn("Vim mode cannot read the screen here", zap.Error(probeErr))
		h.activationRefusal = probeErr
		h.abandonVim(isRefresh)

		return
	}

	if h.system != nil && !h.system.CheckScreenCapturePermission(h.ctx) {
		// The dialog blocks, so it is asked off h.mu and the activation is
		// given up. The user runs the command again once it is granted.
		h.logger.Warn("Vim mode needs the screen recording permission")
		h.activationRefusal = derrors.New(derrors.CodeActionFailed,
			"vim mode needs the screen recording permission, run it again once it is granted")

		system, ctx := h.system, h.ctx
		go system.RequestScreenCapturePermission(ctx)

		h.abandonVim(isRefresh)

		return
	}

	screen := h.vimScreen()
	h.setScreenBounds(screen)

	scope := h.resolveCaptureScope(domain.ModeNameVim, activation, &h.config.Vim)
	region := h.captureStart(domain.ModeNameVim, screen, scope)

	text, err := h.readVimText(region, ports.TextFast)
	if err != nil {
		if derrors.IsNotSupported(err) {
			h.logger.Warn("Vim mode cannot read the screen here", zap.Error(err))
			h.activationRefusal = err
		} else {
			h.logger.Error("Failed to read the screen for vim", zap.Error(err))
		}

		h.abandonVim(isRefresh)

		return
	}

	if text.Len() == 0 {
		h.logger.Warn("No text found on screen for vim")
		h.abandonVim(isRefresh)

		return
	}

	if isRefresh {
		h.vim.host.Replace(text, text.Nearest(h.vimCursorPoint()))
		h.vim.region = region
		h.vim.jump = nil
	} else {
		h.vim = h.newVimSession(text, region)
	}

	h.vim.scope = scope
	h.vim.generation++
	h.vim.reading = h.vision != nil
	h.vim.refined = nil
	h.applyVimActivation(activation, isRefresh)

	if isRefresh {
		h.showFrame(h.vimFrame(), "show vim overlay")
	} else {
		// The surface comes up before the mode is entered, so a backend that
		// cannot draw it refuses the activation instead of holding the
		// keyboard over nothing.
		showErr := h.showFrameResult(h.vimFrame(), "show vim overlay")
		if showErr != nil {
			h.vim = nil
			h.clearOverlayFrame()
			h.activationRefusal = showErr

			return
		}

		h.enterMode(domain.ModeVim)
	}

	h.followVimCursor()

	h.logger.Info("Vim mode activated",
		zap.Duration("elapsed", time.Since(start)),
		zap.Int("line_count", text.Lines()),
		zap.String("scope", scope))

	h.startIndicatorPolling(domain.ModeVim)
	h.refineVim()
}

// abandonVim gives up an activation. A refresh also leaves the mode, or its
// overlay would keep showing text that is no longer on screen.
func (h *handlerState) abandonVim(isRefresh bool) {
	if isRefresh {
		h.exitMode()
	}
}

// vimScreen is the display the session is drawn on: the active screen, or the
// bounds the handler already holds when the platform cannot say.
func (h *handlerState) vimScreen() image.Rectangle {
	if h.system == nil {
		return h.screenBounds
	}

	bounds, err := h.system.ScreenBounds(h.ctx)
	if err != nil {
		if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to get screen bounds for vim", zap.Error(err))
		}

		return h.screenBounds
	}

	return bounds
}

// readVimText reads region at level, bounded the way a hint scan is.
func (h *handlerState) readVimText(
	region image.Rectangle,
	level ports.TextLevel,
) (*screentext.Text, error) {
	if h.vision == nil {
		return nil, derrors.New(derrors.CodeNotSupported, "no vision port to read the screen with")
	}

	ctx, cancel := context.WithTimeout(h.ctx, HintTimeout)
	defer cancel()

	lines, err := h.vision.ReadLines(ctx, region, level)
	if err != nil {
		return nil, err
	}

	return screentext.New(vimWords(lines, region)), nil
}

// refineVim reads the screen again with the accurate recognizer off h.mu and
// swaps the better text in when it lands. One read runs at a time. A read
// asked for while one runs starts when it lands, for the screen as it is then.
func (h *handlerState) refineVim() {
	session := h.vim
	if h.vision == nil || session.refining {
		return
	}

	session.refining = true

	vision, parent, outer := h.vision, h.ctx, h.outer
	generation, region := session.generation, session.region

	// A read still going after vimSlowRead is most likely compiling the
	// model for a script it has not met before, so the overlay says so.
	time.AfterFunc(vimSlowRead, func() { outer.markVimReadSlow(session, generation) })

	go func() {
		ctx, cancel := context.WithTimeout(parent, vimRefineTimeout)
		defer cancel()

		lines, err := vision.ReadLines(ctx, region, ports.TextAccurate)

		// Icons have no text, so the segment detector finds them in the same
		// region. When it fails, the text is kept without icons.
		icons, iconErr := vision.DetectSegments(ctx, region, vimIconTargets)
		if iconErr != nil {
			icons = nil
		}

		outer.applyVimRefine(session, generation, region, lines, icons, err)
	}()
}

// markVimReadSlow shows the slow read notice for the session's accurate read
// of generation, when it has not landed yet.
func (h *Handler) markVimReadSlow(session *vimSession, generation uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.vim != session || session.generation != generation || !session.reading {
		return
	}

	session.slowRead = true

	h.redrawFrame(h.vimFrame(), "update vim overlay")
}

// applyVimRefine keeps an accurate read for the session and the read of the
// screen it was taken for, and swaps it in as soon as the machine is between
// commands. A read for a session since closed, or for a screen since read
// again, is dropped, and the screen as it is now is read instead.
func (h *Handler) applyVimRefine(
	session *vimSession,
	generation uint64,
	region image.Rectangle,
	lines [][]ports.TextPiece,
	icons []*element.Element,
	err error,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.vim != session {
		return
	}

	session.refining = false

	if session.generation != generation {
		h.refineVim()

		return
	}

	session.reading, session.slowRead = false, false

	if err != nil {
		h.logger.Debug("Accurate vim read failed", zap.Error(err))
	} else {
		words := vimWords(lines, region)
		if text := screentext.New(
			append(words, vimIcons(icons, words, region)...),
		); text.Len() > 0 {
			session.refined = text
		}
	}

	if !h.swapInRefinedVimText() {
		h.redrawFrame(h.vimFrame(), "update vim overlay")
	}
}

// swapInRefinedVimText replaces the text with the accurate read when one is
// waiting and nothing would be cut off by it: no command half typed, no
// typing into the app, no jump labels up. It reports whether it redrew.
func (h *handlerState) swapInRefinedVimText() bool {
	session := h.vim
	if session.refined == nil || !session.machine.Idle() || session.typing || session.jump != nil {
		return false
	}

	text := session.refined
	session.refined = nil
	session.host.Replace(text, text.Nearest(h.vimCursorPoint()))
	h.redrawFrame(h.vimFrame(), "update vim overlay")

	return true
}

// rereadVim reads the screen again after vim mode acted on the app, with the
// cursor nearest near, and starts a fresh accurate read.
func (h *handlerState) rereadVim(near image.Point) {
	time.Sleep(vimSettleDelay)

	session := h.vim

	text, err := h.readVimText(session.region, ports.TextFast)
	if err != nil || text.Len() == 0 {
		h.logger.Debug("Vim read after an edit found nothing", zap.Error(err))
		h.redrawFrame(h.vimFrame(), "update vim overlay")

		return
	}

	session.host.Replace(text, text.Nearest(near))
	session.generation++
	session.reading = true
	session.refined = nil

	h.followVimCursor()
	h.redrawFrame(h.vimFrame(), "update vim overlay")
	h.refineVim()
}

// vimWords turns lines read off the screen into words, joining the pieces
// that share a word number. A piece is a whole word, or one of its characters
// when the bridge measured the word's glyphs. Pieces whose center is outside
// region are dropped.
func vimWords(lines [][]ports.TextPiece, region image.Rectangle) []screentext.Word {
	var words []screentext.Word

	for _, line := range lines {
		var word *screentext.Word

		exact, number := true, -1

		for _, piece := range line {
			if !region.Empty() &&
				!piece.Bounds.Min.Add(piece.Bounds.Size().Div(vimHalf)).In(region) {
				continue
			}

			if word == nil || piece.Word != number {
				if word != nil {
					words = append(words, finishVimWord(*word, exact))
				}

				word = &screentext.Word{Bounds: piece.Bounds, Background: piece.Background}
				exact, number = true, piece.Word
			}

			chars := len(screentext.Characters(piece.Text))
			word.Text += piece.Text
			word.Bounds = word.Bounds.Union(piece.Bounds)

			// Only a piece of one character says where that character is.
			// A word piece leaves its characters a share of the word.
			for range chars {
				word.Boxes = append(word.Boxes, piece.Bounds)
			}

			if chars != 1 {
				exact = false
			}
		}

		if word != nil {
			words = append(words, finishVimWord(*word, exact))
		}
	}

	return words
}

// vimIconTargets asks the segment detector for controls, icons and pictures,
// and no text, since the text is read already.
var vimIconTargets = configpkg.HintsSegmentConfig{Targets: domain.SegmentTargetsControls}

const (
	// vimIconMaxSize is the widest or tallest shape, in points, read as an
	// icon. A bigger one is a panel or a large picture, and stays out of the
	// text.
	vimIconMaxSize = 160
	// vimIconMaxTextShare is how much of a shape words may cover, in percent,
	// before it counts as the button around them rather than an icon.
	vimIconMaxTextShare = 20
	// vimPercent turns a share into percent.
	vimPercent = 100
)

// vimIcons keeps the shapes the segment detector found that are icons: in
// region, no bigger than vimIconMaxSize, and not mostly covered by words,
// which already reach them.
func vimIcons(
	elements []*element.Element,
	words []screentext.Word,
	region image.Rectangle,
) []screentext.Word {
	var icons []screentext.Word

	for _, candidate := range elements {
		bounds := candidate.Bounds()
		if bounds.Empty() || !candidate.Center().In(region) ||
			bounds.Dx() > vimIconMaxSize || bounds.Dy() > vimIconMaxSize {
			continue
		}

		covered := 0
		for _, word := range words {
			overlap := bounds.Intersect(word.Bounds).Size()
			covered += overlap.X * overlap.Y
		}

		area := bounds.Dx() * bounds.Dy()
		if covered*vimPercent > area*vimIconMaxTextShare {
			continue
		}

		icons = append(icons, screentext.Icon(bounds))
	}

	return icons
}

// finishVimWord drops a word's boxes unless every one came from a piece of
// one character.
func finishVimWord(word screentext.Word, exact bool) screentext.Word {
	if !exact {
		word.Boxes = nil
	}

	return word
}

// newVimSession puts the cursor on the character nearest the pointer and
// gives the machine the screen's operators, ex commands and edits.
func (h *handlerState) newVimSession(text *screentext.Text, region image.Rectangle) *vimSession {
	pos := 0

	pointer, err := h.actionService.CursorPosition(h.ctx)
	if err == nil {
		pos = text.Nearest(pointer)
	}

	session := &vimSession{region: region}
	session.host = screentext.NewHost(text, pos, func(s string) { session.yank = s })
	session.machine = screentext.NewMachine(session.host)

	clickOperator := func(name string) {
		session.machine.Operator(name, 0,
			func(_ vim.Text, start, end int, _ bool, _ []string) []vim.Edit {
				if p, ok := rangeCenter(session.host.Screen(), start, end); ok {
					session.lastInsert = nil
					session.act = func() { h.vimSelect(p, name) }
				}

				return nil
			})
	}

	clickOperator(vimOperatorClick)
	clickOperator(vimOperatorRightClick)
	clickOperator(vimOperatorMiddleClick)

	session.machine.Operator(vimOperatorDrag, 0,
		func(_ vim.Text, start, end int, _ bool, _ []string) []vim.Edit {
			session.act = func() {
				h.vimDragRange(start, end)
				h.exitMode()
			}

			return nil
		})

	session.host.Ex = func(line string) error {
		switch strings.TrimSpace(strings.TrimPrefix(line, ":")) {
		case "q", "quit":
			session.act = h.exitMode
		case "click":
			session.act = func() { h.vimSelectAtCursor(vimOperatorClick) }
		case "rightclick":
			session.act = func() { h.vimSelectAtCursor(vimOperatorRightClick) }
		case "middleclick":
			session.act = func() { h.vimSelectAtCursor(vimOperatorMiddleClick) }
		default:
			return screentext.ErrNotACommand
		}

		return nil
	}

	session.host.OnApply = func(edits []vim.Edit, label string) error {
		if session.edited {
			return errVimOneEdit
		}

		if len(edits) > vimMaxEdits ||
			vimEditRunes(session.host.Screen(), edits) > vimMaxSelectRunes {
			return errVimTooBig
		}

		session.edited = true
		session.edits = append(session.edits[:0], edits...)
		session.label = label
		session.before = session.host.Screen()

		return nil
	}

	session.host.OnUndo = func() { session.act = func() { h.vimUndo(vimKeyUndo) } }
	session.host.OnRedo = func() { session.act = func() { h.vimUndo(vimKeyRedo) } }

	return session
}

var (
	// errVimOneEdit refuses a second batch of edits in one key.
	errVimOneEdit = derrors.New(derrors.CodeInvalidInput, "one edit at a time, the screen moved")
	// errVimTooBig refuses an edit that would take the app seconds to make,
	// with the keyboard held all the while.
	errVimTooBig = derrors.New(derrors.CodeInvalidInput, "too much to change at once")
)

const (
	// vimMaxEdits is the most places one command may change, as :s/a/b/g does.
	vimMaxEdits = 64
	// vimMaxSelectRunes is the most characters one command may select in the
	// app, one Shift+Right each.
	vimMaxSelectRunes = 2000
)

// vimEditRunes counts the characters the app would select to make edits.
func vimEditRunes(text *screentext.Text, edits []vim.Edit) int {
	count := 0
	for _, e := range edits {
		count += text.AppChars(e.Start, e.End)
	}

	return count
}

// The operators vim mode adds, which act on the screen rather than the text.
const (
	vimOperatorClick       = "gc"
	vimOperatorRightClick  = "gr"
	vimOperatorMiddleClick = "gm"
	vimOperatorDrag        = "gs"
)

// vimOperatorActions is the mouse action each click operator runs.
var vimOperatorActions = map[string]string{
	vimOperatorClick:       string(action.NameLeftClick),
	vimOperatorRightClick:  string(action.NameRightClick),
	vimOperatorMiddleClick: string(action.NameMiddleClick),
}

// applyVimActivation keeps what the activation asked for. A refresh keeps
// what the session had for anything it leaves out.
func (h *handlerState) applyVimActivation(activation modecmd.Activation, isRefresh bool) {
	session := h.vim

	if !isRefresh || activation.CursorFollowSelection != nil {
		session.follow = resolveCursorFollowSelection(activation.CursorFollowSelection)
	}

	if !isRefresh || activation.Action != nil {
		session.action = activation.Action
		session.modifier = activation.Modifier
		session.onExit = activation.OnExit
	}

	if activation.Repeat != nil {
		session.repeat = *activation.Repeat
	}
}

// vimSessionActivation is the activation that reads the screen again for the
// open session, keeping what it was opened with.
func (h *handlerState) vimSessionActivation() modecmd.Activation {
	scope := h.vim.scope

	return modecmd.Activation{Mode: domain.ModeVim, CaptureScope: &scope}
}

// rangeCenter is the middle of the first line of [start, end).
func rangeCenter(text *screentext.Text, start, end int) (image.Point, bool) {
	r, ok := lineRect(text, start, max(end, start+1))
	if !ok {
		return image.Point{}, false
	}

	return r.Min.Add(r.Size().Div(vimHalf)), true
}

// lineRect covers the bytes of [start, end) on start's line.
func lineRect(text *screentext.Text, start, end int) (image.Rectangle, bool) {
	first, ok := text.Box(start)
	if !ok {
		return image.Rectangle{}, false
	}

	rect := first

	for pos := start + 1; pos < end; pos++ {
		if text.Slice(pos, pos+1) == "\n" {
			break
		}

		if r, ok := text.Box(pos); ok {
			rect = rect.Union(r)
		}
	}

	return rect, true
}

// vimCursorPoint is the middle of the character under the cursor.
func (h *handlerState) vimCursorPoint() image.Point {
	p, _ := h.vim.host.Screen().Point(h.vim.host.Cursor())

	return p
}

// followVimCursor moves the real cursor onto the vim cursor when the session
// follows it.
func (h *handlerState) followVimCursor() {
	if !h.vim.follow {
		return
	}

	err := h.actionService.MoveCursorToPoint(h.ctx, h.vimCursorPoint())
	if err != nil {
		h.logger.Debug("Failed to move the cursor to the vim cursor", zap.Error(err))
	}
}

// vimFrame describes the session as it stands.
func (h *handlerState) vimFrame() ports.VimFrame {
	session := h.vim
	text := session.host.Screen()
	machine := session.machine
	cursor := session.host.Cursor()

	frame := ports.VimFrame{
		Screen:  h.screenBounds,
		Region:  session.region,
		Pending: machine.Pending(),
		Current: -1,
	}

	if session.typing {
		frame.Status = "INSERT   typing into the app, Escape to stop"

		return frame
	}

	if index, ok := text.ContainerAt(cursor); ok && len(text.Containers()) > 1 {
		frame.Container = text.Containers()[index].Bounds
	}

	if r, ok := text.Box(cursor); ok {
		frame.Cursor = r
	}

	if sel, ok := machine.Selection(); ok {
		for _, seg := range lineSegments(text, sel.Start, sel.End) {
			if r, ok := lineRect(text, seg[0], seg[1]); ok {
				frame.Selection = append(frame.Selection, r)
			}
		}
	}

	if jump := session.jump; jump != nil {
		for index, match := range jump.matches {
			if r, ok := lineRect(text, match[0], match[1]); ok {
				frame.Matches = append(frame.Matches, r)
			}

			if r, ok := vimJumpLabelBox(text, match); ok && jump.labels[index] != "" {
				frame.Labels = append(
					frame.Labels,
					ports.VimLabel{Text: jump.labels[index], Bounds: r},
				)
			}
		}
	} else {
		current, _, typing := machine.Incremental()

		for index, match := range machine.Matches(text, 0, text.Len()) {
			if r, ok := lineRect(text, match[0], match[1]); ok {
				frame.Matches = append(frame.Matches, r)
			}

			if (typing && match[0] == current) ||
				(!typing && cursor >= match[0] && cursor < match[1]) {
				frame.Current = index
			}
		}
	}

	if session.reading && session.slowRead {
		frame.Notice = vimSlowReadNotice
	}

	frame.Selection = append(frame.Selection, session.flash...)
	frame.Status = vimStatus(session, frame.Current, len(frame.Matches))
	frame.Lens = vimLens(text, cursor)

	return frame
}

// vimSlowReadNotice is what the overlay says while an accurate read takes
// long, which on macOS is a model compiling for a script met the first time.
const vimSlowReadNotice = "Preparing text recognition\n" +
	"First time for this language, up to 15 seconds. Moving around works meanwhile."

// lineSegments splits [start, end) at line breaks.
func lineSegments(text *screentext.Text, start, end int) [][2]int {
	var segs [][2]int

	for start < end {
		lineEnd := end
		if i := strings.IndexByte(text.Slice(start, end), '\n'); i >= 0 {
			lineEnd = start + i
		}

		if lineEnd > start {
			segs = append(segs, [2]int{start, lineEnd})
		}

		start = lineEnd + 1
	}

	return segs
}

// vimStatus is the status bar: the mode, the command being typed or the
// command line, the machine's message, a macro being recorded, where the
// cursor is among the search matches, and whether the accurate read is
// still running.
func vimStatus(session *vimSession, current, matches int) string {
	machine := session.machine
	parts := []string{vimModeName(session)}

	switch {
	case session.jump != nil:
		parts = append(parts, "s"+session.jump.query)
	case machine.Mode() == vim.Cmdline:
		line := machine.Line()
		at := min(max(machine.LineCursor(), 0), len(line))
		parts = append(parts, line[:at]+"▏"+line[at:])
	case machine.Message() != "":
		parts = append(parts, machine.Message())
	case session.notice != "":
		parts = append(parts, session.notice)
	case session.zPrefix && session.zCount > 0:
		parts = append(parts, strconv.Itoa(session.zCount)+"z")
	case session.zPrefix:
		parts = append(parts, "z")
	case machine.Shown() != "":
		parts = append(parts, machine.Shown())
	}

	if r := machine.Recording(); r != 0 {
		parts = append(parts, "recording @"+string(r))
	}

	if matches > 0 {
		at := "-"
		if current >= 0 {
			at = strconv.Itoa(current + 1)
		}

		parts = append(parts, "["+at+"/"+strconv.Itoa(matches)+"]")
	}

	if session.reading {
		parts = append(parts, "reading…")
	}

	return strings.Join(parts, "   ")
}

// vimModeName names the session's mode for the status bar.
func vimModeName(session *vimSession) string {
	machine := session.machine

	if session.jump != nil {
		return "JUMP"
	}

	if sel, ok := machine.Selection(); ok {
		switch sel.Kind {
		case "V":
			return "V-LINE"
		case "<C-v>":
			return "V-BLOCK"
		default:
			return "VISUAL"
		}
	}

	switch {
	case machine.Mode() == vim.Cmdline:
		return "COMMAND"
	case machine.Pending():
		return "PENDING"
	default:
		return "NORMAL"
	}
}

// vimLens is the word under the cursor as it was read, split around the
// cursor's character. A cursor on a blank shows no lens.
func vimLens(text *screentext.Text, cursor int) ports.VimLens {
	all := text.String()
	if cursor < 0 || cursor >= len(all) {
		return ports.VimLens{}
	}

	r, size := utf8.DecodeRuneInString(all[cursor:])
	if unicode.IsSpace(r) {
		return ports.VimLens{}
	}

	start := cursor
	for start > 0 {
		prev, n := utf8.DecodeLastRuneInString(all[:start])
		if unicode.IsSpace(prev) {
			break
		}

		start -= n
	}

	end := cursor + size
	for end < len(all) {
		next, n := utf8.DecodeRuneInString(all[end:])
		if unicode.IsSpace(next) {
			break
		}

		end += n
	}

	return ports.VimLens{
		Before: all[start:cursor],
		At:     all[cursor : cursor+size],
		After:  all[cursor+size : end],
	}
}
