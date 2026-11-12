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
	// vimRefineTimeout bounds the accurate read that follows an activation,
	// counted from once the vision port has prepared it.
	vimRefineTimeout = 30 * time.Second
	// vimSettleDelay is how long the app gets to redraw after vim mode acted
	// on it, before the screen is read again.
	vimSettleDelay = 80 * time.Millisecond
	// vimHalf halves a size to find a middle.
	vimHalf = 2
)

// vimRefineLetFinish is how long an accurate read runs before a newer screen
// stops canceling it. A warm read takes well under a second. One running
// longer is allowed to finish, so a screen that never stops changing still
// gets an accurate read.
const vimRefineLetFinish = 1500 * time.Millisecond

// vimSession is an open vim mode: the text read off the screen, the machine
// moving a cursor over it, and what the activation asked for.
type vimSession struct {
	machine *vim.Machine
	host    *screentext.Host
	region  image.Rectangle // what was read, global
	// shown is the region the overlay draws over, global. It is region with
	// the strip the status bar covers below it, which vim does not read.
	shown image.Rectangle

	scope  string
	follow bool

	// completion is Tab cycling through the ex commands on the command line,
	// nil once any other key is typed.
	completion *vimCompletion

	// What a key asked of the app, done in order once Feed returns and its
	// edits are made, since the machine is still in the middle of the key
	// when it calls back. One mapping can run several.
	acts  []func()
	edits []vim.Edit
	// before is the text the edits' offsets are in, as it was on screen.
	before *screentext.Text
	yank   string
	// edited reports that this key already handed over edits. A second batch
	// in the same key, from a count or a macro, would land on offsets the
	// first one moved, so it is refused.
	edited bool

	// typing is insert mode: keys go to the app, until Escape.
	typing bool
	// typedAt is where typing began, which places the cursor after it.
	typedAt image.Point
	// eraseOnEscape is a change whose text is still selected in the app.
	// The first key typed replaces it, and Escape before any erases it.
	eraseOnEscape bool

	// generation counts reads, so a late accurate read of an older screen is
	// dropped. reading is true until the accurate read lands, and refined
	// holds it while the machine is mid-command. fastLines are the lines of
	// the last fast read, which stand in for the accurate read where it would
	// read the same.
	generation uint64
	reading    bool
	refined    *screentext.Text
	fastLines  [][]ports.TextPiece
	// refining is an accurate read in flight. One runs at a time, and a read
	// asked for meanwhile waits for it. preparing is set while it waits for
	// the vision port to prepare accurate reads, and the overlay says so.
	refining  bool
	preparing bool
	// refineCancel stops the accurate read in flight, which began at
	// refineStarted.
	refineCancel  context.CancelFunc
	refineStarted time.Time

	// scrolled is when the last scroll of the app started. stale is set from
	// a scroll until the screen is read again, staleNear is where the
	// cursor's text went, and afterScroll is a key that came to j or k,
	// scrolled at an edge and still has to move the cursor. scrollSeq tells
	// the read of a scrolled screen whether another scroll came after it,
	// readCancel stops that read, and readDone closes once it has stopped
	// capturing and recognizing.
	scrolled    time.Time
	stale       bool
	staleNear   image.Point
	afterScroll vim.Key
	scrollSeq   uint64
	readCancel  context.CancelFunc
	readDone    <-chan struct{}

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

	// keymap is what the machine maps, and held the keys it holds because
	// they start a longer mapping. mapTimer runs them as typed when the rest
	// of the mapping does not come within vimMapTimeout, and mapWait tells
	// it whether a key came since it started.
	keymap   screentext.Keymap
	held     []vim.Key
	mapTimer *time.Timer
	mapWait  uint64

	// flash is where the last yank took its text, shown briefly, and notice
	// what the status bar says about it until the next key. flashSeq tells
	// a flash's timer whether a later yank replaced it.
	flash    []image.Rectangle
	flashSeq uint64
	notice   string
}

// startVim reads the screen and opens vim mode over it. Activated again while
// open, it reads the screen again and keeps the machine, so registers,
// macros and the last search carry over, with the cursor put back
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
	_, _, probeErr := h.readVimText(image.Rectangle{})
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
	shown := h.captureStart(domain.ModeNameVim, screen, scope)
	region := h.vimReadRegion(shown)

	if isRefresh {
		h.stopScrolledRead()
	}

	text, lines, err := h.readVimText(region)
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
		h.replaceVimText(text, text.Nearest(h.vimCursorPoint()))
		h.vim.region = region
		h.vim.jump = nil
	} else {
		h.vim = h.newVimSession(text, region)
	}

	h.vim.shown = shown
	h.vim.scope = scope
	h.vim.generation++
	h.vim.reading = h.vision != nil
	h.vim.refined = nil
	h.vim.fastLines = lines
	h.vim.stale, h.vim.afterScroll = false, ""
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

// vimReadRegion is what vim reads of shown, the region the overlay draws
// over. It is all of shown above the strip the status bar covers. The bar
// hides the text under it while vim is up, and leaving the bar out of the read
// keeps a Linux capture from taking it off the screen.
func (h *handlerState) vimReadRegion(shown image.Rectangle) image.Rectangle {
	if h.overlayPort == nil {
		return shown
	}

	bar := h.overlayPort.VimStatusBar(shown)
	if bar.Empty() || bar.Min.Y <= shown.Min.Y {
		return shown
	}

	return image.Rect(shown.Min.X, shown.Min.Y, shown.Max.X, bar.Min.Y)
}

// readVimText reads region with the fast recognizer, bounded the way a hint
// scan is, and returns the text with the lines it was made from.
func (h *handlerState) readVimText(
	region image.Rectangle,
) (*screentext.Text, [][]ports.TextPiece, error) {
	if h.vision == nil {
		return nil, nil, derrors.New(
			derrors.CodeNotSupported,
			"no vision port to read the screen with",
		)
	}

	ctx, cancel := context.WithTimeout(h.ctx, HintTimeout)
	defer cancel()

	lines, err := h.vision.ReadLines(ctx, region, ports.TextFast)
	if err != nil {
		return nil, nil, err
	}

	return screentext.New(vimWords(lines, region)), lines, nil
}

// refineVim reads the screen again with the accurate recognizer off h.mu and
// swaps the better text in when it lands. Where the accurate read would read
// what the fast one did, the fast read's lines stand in, and only the icons
// are looked for. One read runs at a time. A read asked for while one runs
// stops it when it is young (cancelYoungRefine), and starts once it lands.
func (h *handlerState) refineVim() {
	if h.vision == nil {
		return
	}

	if h.vim.refining {
		h.cancelYoungRefine()

		return
	}

	h.startRefine()
}

// startRefine starts the accurate read of the session's last fast read.
func (h *handlerState) startRefine() {
	session := h.vim
	session.refining = true
	session.refineStarted = time.Now()

	vision, parent, outer := h.vision, h.ctx, h.outer
	generation, region, fastLines := session.generation, session.region, session.fastLines

	ctx, cancel := context.WithCancel(parent)
	session.refineCancel = cancel

	ready := vision.PrepareAccurateRead()
	select {
	case <-ready:
		session.preparing = false
	default:
		session.preparing = true

		h.redrawFrame(h.vimFrame(), "update vim overlay")
	}

	waits := session.preparing

	go func() {
		defer cancel()

		select {
		case <-ready:
		case <-ctx.Done():
			outer.applyVimRefine(session, generation, region, nil, nil, ctx.Err())

			return
		}

		if waits {
			outer.endVimRefineWait(session, generation)
		}

		readCtx, stop := context.WithTimeout(ctx, vimRefineTimeout)
		defer stop()

		lines, err := fastLines, error(nil)
		if vision.AccurateReadDiffers() {
			lines, err = vision.ReadLines(readCtx, region, ports.TextAccurate)
		}

		// Icons have no text, so the segment detector finds them in the same
		// region. When it fails, the text is kept without icons.
		icons, iconErr := vision.DetectSegmentsAsRead(readCtx, region, vimIconTargets)
		if iconErr != nil {
			icons = nil
		}

		outer.applyVimRefine(session, generation, region, lines, icons, err)
	}()
}

// cancelYoungRefine stops the accurate read in flight, for a screen that has
// since changed, while it is still waiting for the vision port to prepare or
// has run less than vimRefineLetFinish. Canceling the wait leaves the
// preparation running. Canceling stops a tesseract, Windows OCR or macOS
// Vision recognition, the macOS helper process and the icon pass.
func (h *handlerState) cancelYoungRefine() {
	session := h.vim
	if session.refining &&
		(session.preparing || time.Since(session.refineStarted) < vimRefineLetFinish) {
		session.refineCancel()
	}
}

// endVimRefineWait takes the notice down once the session's accurate read of
// generation has stopped waiting for the vision port, and restarts the read's
// age, so cancelYoungRefine judges it by how long it has been reading.
func (h *Handler) endVimRefineWait(session *vimSession, generation uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.vim != session || session.generation != generation || !session.preparing {
		return
	}

	session.preparing = false
	session.refineStarted = time.Now()

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

	session.refining, session.preparing, session.refineCancel = false, false, nil

	if session.generation != generation {
		if !session.stale {
			h.refineVim()
		}

		return
	}

	session.reading = false

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

	h.replaceVimText(text, text.Nearest(h.vimCursorPoint()))
	h.redrawFrame(h.vimFrame(), "update vim overlay")

	return true
}

// rereadVim reads the screen again after vim mode acted on the app, once it
// has had vimSettleDelay to redraw.
func (h *handlerState) rereadVim(near image.Point) {
	time.Sleep(vimSettleDelay)
	h.readVimAgain(near)
}

// readVimAfterScroll reads the screen a scroll moved, off h.mu, once the app
// has moved it and it has stopped moving (ReadLinesOnceChanged), and lands
// through applyScrolledVim. The next scroll cancels it, and a key that needs
// the text first reads it itself (readScrolledVim).
func (h *handlerState) readVimAfterScroll() {
	session := h.vim

	previous := h.cancelScrolledRead()

	vision, outer, region, seq := h.vision, h.outer, session.region, session.scrollSeq

	ctx, cancel := context.WithTimeout(h.ctx, HintTimeout)
	done := make(chan struct{})
	session.readCancel, session.readDone = cancel, done

	go func() {
		defer cancel()

		// The read it replaces may still be capturing, and KDE's screen-cast
		// session refuses a second capture meanwhile.
		if previous != nil {
			<-previous
		}

		lines, err := vision.ReadLinesOnceChanged(ctx, region)

		close(done)
		outer.applyScrolledVim(session, seq, lines, err)
	}()
}

// applyScrolledVim lands the read of the screen the scroll numbered seq left.
// One for a session since closed, or a screen scrolled again or already read
// for a key, is dropped.
func (h *Handler) applyScrolledVim(
	session *vimSession,
	seq uint64,
	lines [][]ports.TextPiece,
	err error,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.vim != session || !session.stale || session.scrollSeq != seq {
		return
	}

	h.finishScrolledVim(lines, err)
}

// readScrolledVim reads the screen the scrolls left, now and under h.mu, for a
// key that acts on the text. The read already running is canceled, since this one
// replaces it.
func (h *handlerState) readScrolledVim() bool {
	session := h.vim

	h.stopScrolledRead()

	ctx, cancel := context.WithTimeout(h.ctx, HintTimeout)
	defer cancel()

	lines, err := h.vision.ReadLinesOnceChanged(ctx, session.region)

	return h.finishScrolledVim(lines, err)
}

// finishScrolledVim takes the read of the screen the scrolls left, and moves the
// cursor the line a j or k that scrolled at an edge asked for. It reports
// false when the read found nothing, so the text is still the screen before
// the scroll.
func (h *handlerState) finishScrolledVim(lines [][]ports.TextPiece, err error) bool {
	session := h.vim

	var text *screentext.Text
	if err == nil {
		text = screentext.New(vimWords(lines, session.region))
	}

	// The text held is still for the screen before the scroll, so a read
	// that found nothing leaves the session stale, and the next key reads
	// again.
	if err != nil || text.Len() == 0 {
		h.logger.Debug("Vim read after a scroll found nothing", zap.Error(err))

		return false
	}

	key := session.afterScroll
	session.stale, session.afterScroll = false, ""

	h.applyVimRead(session.staleNear, text, lines, nil)

	// A key held for a longer mapping was typed after the j or k, and
	// feeding the j or k now would join the hold, so its move is dropped.
	if key != "" && h.vim == session && !session.machine.Waiting() && h.feedVim(key) {
		h.followVimCursor()
		h.redrawFrame(h.vimFrame(), "update vim overlay")
	}

	return true
}

// vimNoticeUnread is the status bar's notice for a key dropped because the
// screen a scroll left could not be read, so the text is the one before it.
const vimNoticeUnread = "could not read the screen after the scroll"

// cancelScrolledRead stops the read of a scrolled screen in flight. It returns
// a channel that closes once the read has stopped capturing and recognizing,
// or nil when none was running.
func (h *handlerState) cancelScrolledRead() <-chan struct{} {
	session := h.vim
	if session.readCancel == nil {
		return nil
	}

	session.readCancel()

	done := session.readDone
	session.readCancel, session.readDone = nil, nil

	return done
}

// stopScrolledRead cancels the read of a scrolled screen in flight and waits
// for it to stop capturing, for a caller that captures next. A canceled
// capture can still hold KDE's screen-cast session, which refuses a second
// one. The wait is safe under h.mu, because the read closes readDone before it
// takes h.mu to land. It lasts as long as the capture or recognition step the
// read was in.
func (h *handlerState) stopScrolledRead() {
	if done := h.cancelScrolledRead(); done != nil {
		<-done
	}
}

// readVimAgain reads the screen again with the cursor nearest near.
func (h *handlerState) readVimAgain(near image.Point) {
	text, lines, err := h.readVimText(h.vim.region)
	h.applyVimRead(near, text, lines, err)
}

// replaceVimText puts a new read of the screen in place of the text, with
// the cursor at pos. Visual mode ends first, because its anchor is an offset
// into the old text, which in the new one is other text or past its end.
func (h *handlerState) replaceVimText(text *screentext.Text, pos int) {
	if h.vim.machine.Mode() == vim.Visual {
		h.feedVim(vimEscape)
	}

	h.vim.host.Replace(text, pos)
}

// applyVimRead replaces the text with a read of the screen, the cursor
// nearest near, and starts a fresh accurate read.
func (h *handlerState) applyVimRead(
	near image.Point,
	text *screentext.Text,
	lines [][]ports.TextPiece,
	err error,
) {
	session := h.vim

	if err != nil || text.Len() == 0 {
		h.logger.Debug("Vim read after acting on the app found nothing", zap.Error(err))
		h.redrawFrame(h.vimFrame(), "update vim overlay")

		return
	}

	h.replaceVimText(text, text.Nearest(near))

	session.generation++
	session.reading = true
	session.refined = nil
	session.fastLines = lines

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

	for index, line := range lines {
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

				word = &screentext.Word{
					Bounds:      piece.Bounds,
					Background:  piece.Background,
					Line:        index + 1,
					BorderAbove: piece.BorderAbove,
					BorderBelow: piece.BorderBelow,
				}
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

	session.machine.Operator(screentext.OperatorDrag, 0,
		func(_ vim.Text, start, end int, _ bool, _ []string) []vim.Edit {
			session.queue(func() {
				h.vimDragRange(start, end)
				h.exitMode()
			})

			return nil
		})

	session.host.Ex = func(line string) error {
		command, arg, _ := strings.Cut(strings.TrimSpace(strings.TrimPrefix(line, ":")), " ")
		arg = strings.TrimSpace(arg)

		if command == "q" || command == "quit" {
			if arg != "" {
				return screentext.ErrNotACommand
			}

			session.queue(h.exitMode)

			return nil
		}

		if name, ok := vimExMouse[command]; ok && arg == "" {
			session.queue(func() { h.vimMouseAtCursor(name, 1) })

			return nil
		}

		button, ok := vimExClicks[command]
		if !ok {
			return screentext.ErrNotACommand
		}

		times := 1

		if arg != "" {
			n, err := strconv.Atoi(arg)
			if err != nil || n < 1 {
				return errVimClickCount
			}

			times = n
		}

		session.queue(func() { h.vimMouseAtCursor(button, times) })

		return nil
	}

	session.host.OnApply = func(edits []vim.Edit, _ string) error {
		if session.edited {
			return errVimOneEdit
		}

		// Every edit reaches the app as typed keys, a put's text included,
		// so text with a character no key types is refused before the
		// machine changes anything.
		typed := 0

		for _, edit := range edits {
			keys, typeable := vimEditKeys(edit.Text)
			if !typeable {
				return errVimUntypeable
			}

			typed += len(keys)
		}

		if len(edits) > vimMaxEdits ||
			vimEditRunes(session.host.Screen(), edits)+typed > vimMaxEditKeys {
			return errVimTooBig
		}

		session.edited = true
		session.edits = append(session.edits[:0], edits...)
		session.before = session.host.Screen()

		return nil
	}

	// Where a typed key cannot reach the app, every change is refused before
	// anything is clicked or typed, and the machine says why.
	if gate, ok := h.keyFeed.(ports.KeyFeedGate); ok {
		session.host.CanEdit = gate.ReachesApp
	}

	// A count calls these once per step, so 3u undoes three times in the app.
	session.host.OnUndo = func() { session.queue(func() { h.vimUndo(vimKeyUndo) }) }
	session.host.OnRedo = func() { session.queue(func() { h.vimUndo(vimKeyRedo) }) }

	h.addVimPlugs(session)

	user := screentext.Keymap{
		vim.Normal: h.config.Vim.Keymaps.Normal,
		vim.Visual: h.config.Vim.Keymaps.Visual,
	}
	session.keymap = screentext.DefaultKeymap().With(user)
	user.Apply(session.machine)

	return session
}

// queue adds act to what the key asks of the app. Past vimMaxActs it is
// refused, and the status bar says so.
func (s *vimSession) queue(act func()) {
	if len(s.acts) >= vimMaxActs {
		s.notice = derrors.Message(errVimTooBig)

		return
	}

	s.acts = append(s.acts, act)
}

// addVimPlugs gives the machine vim mode's commands, each run by its <Plug>
// key. They act on the app, so each queues its action for once the key is
// done.
func (h *handlerState) addVimPlugs(session *vimSession) {
	plug := func(name string, run func(count int)) {
		session.machine.Command(name, func(_ vim.Text, _, count int) ([]vim.Edit, string) {
			session.queue(func() { run(count) })

			return nil, ""
		})
	}

	plug(screentext.PlugJump, func(int) { h.startVimJump() })
	plug(screentext.PlugReadScreen, func(int) { h.startVim(h.vimSessionActivation()) })
	plug(screentext.PlugCenterLine, func(int) {
		h.followVimCursor()
		h.vimCenterLine()
	})

	// A count clicks that many times, as 2gc double clicks.
	for name, button := range vimMousePlugs {
		plug(name, func(count int) { h.vimMouseAtCursor(button, count) })
	}

	// A count scrolls that many characters sideways, as 3zl does.
	for name, scroll := range vimScrollPlugs {
		plug(name, func(count int) {
			counted := scroll
			if count > 0 && counted.columns > 0 {
				counted.columns = count
			}

			h.vimScroll(counted)
		})
	}

	// The jump lands with go, the machine's own motion to a byte, typed
	// through no mapping.
	session.machine.Command(vimPlugGoName, func(vim.Text, int, int) ([]vim.Edit, string) {
		return nil, "go"
	})
}

// vimMaxActs is the most actions one key may queue, as a mapping or a macro
// running vim mode's commands. Each can click and read the screen, all with
// the keyboard held.
const vimMaxActs = 8

// vimPlugGoName names the command that runs go, for vim mode itself rather
// than a mapping, and vimPlugGo is its key.
const (
	vimPlugGoName         = "neru-go"
	vimPlugGo     vim.Key = "<Plug>(" + vimPlugGoName + ")"
)

var (
	// errVimOneEdit refuses a second batch of edits in one key.
	errVimOneEdit = derrors.New(derrors.CodeInvalidInput, "one edit at a time, the screen moved")
	// errVimTooBig refuses an edit that would take the app seconds to make,
	// with the keyboard held all the while.
	errVimTooBig = derrors.New(derrors.CodeInvalidInput, "too much to change at once")
	// errVimUntypeable refuses an edit whose text has a character vim mode
	// has no key to type, such as a letter outside ASCII.
	errVimUntypeable = derrors.New(
		derrors.CodeInvalidInput,
		"the text has a character vim mode cannot type",
	)
	// errVimClickCount refuses a click command whose argument is not a
	// number of clicks.
	errVimClickCount = derrors.New(
		derrors.CodeInvalidInput,
		"a click takes a number, as :leftclick 2",
	)
)

const (
	// vimMaxEdits is the most places one command may change, as :s/a/b/g does.
	vimMaxEdits = 64
	// vimMaxEditKeys is the most keys one command's edits may send the app:
	// a Shift+Right for each character selected and a key for each typed.
	vimMaxEditKeys = 2000
)

// vimEditRunes counts the characters the app would select to make edits.
func vimEditRunes(text *screentext.Text, edits []vim.Edit) int {
	count := 0
	for _, e := range edits {
		count += text.AppChars(e.Start, e.End)
	}

	return count
}

// vimMaxClicks is the most times a count repeats a click, which a triple
// click needs. A larger count clicks this many times.
const vimMaxClicks = 3

// vimMousePlugs are vim mode's commands that act at the cursor, and the
// action each runs: three click, as gc does, and one moves the real cursor
// there.
var vimMousePlugs = map[string]action.Name{
	screentext.PlugLeftClick:   action.NameLeftClick,
	screentext.PlugRightClick:  action.NameRightClick,
	screentext.PlugMiddleClick: action.NameMiddleClick,
	screentext.PlugMoveMouse:   action.NameMoveMouse,
}

// vimExClicks are the ex commands that click at the cursor, as :leftclick 2
// double clicks, and the button each clicks.
var vimExClicks = map[string]action.Name{
	"leftclick":   action.NameLeftClick,
	"rightclick":  action.NameRightClick,
	"middleclick": action.NameMiddleClick,
}

// vimExMouse are the ex commands that act once at the cursor, and the action
// each runs. They press or release a button, so a motion between them drags,
// or move the real cursor there.
var vimExMouse = map[string]action.Name{
	"movemouse":  action.NameMoveMouse,
	"leftdown":   action.NameLeftMouseDown,
	"leftup":     action.NameLeftMouseUp,
	"rightdown":  action.NameRightMouseDown,
	"rightup":    action.NameRightMouseUp,
	"middledown": action.NameMiddleMouseDown,
	"middleup":   action.NameMiddleMouseUp,
}

// vimExNames are the ex commands Tab completes, vim mode's own and the
// machine's, sorted. Commands named with symbols, such as &&, are left out.
func vimExNames() []string {
	names := []string{"q", "quit"}

	for name := range vimExClicks {
		names = append(names, name)
	}

	for name := range vimExMouse {
		names = append(names, name)
	}

	for _, command := range vim.Commands() {
		if strings.TrimFunc(command.Name, unicode.IsLetter) == "" {
			names = append(names, command.Name)
		}
	}

	slices.Sort(names)

	return slices.Compact(names)
}

// applyVimActivation keeps what the activation asked for. A refresh keeps
// what the session had for anything it leaves out.
func (h *handlerState) applyVimActivation(activation modecmd.Activation, isRefresh bool) {
	session := h.vim

	if !isRefresh || activation.CursorFollowSelection != nil {
		session.follow = resolveCursorFollowSelection(activation.CursorFollowSelection)
	}
}

// vimSessionActivation is the activation that reads the screen again for the
// open session, keeping what it was opened with.
func (h *handlerState) vimSessionActivation() modecmd.Activation {
	scope := h.vim.scope

	return modecmd.Activation{Mode: domain.ModeVim, CaptureScope: &scope}
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
	if h.vim.follow {
		h.moveToVimCursor()
	}
}

// moveToVimCursor moves the real cursor onto the vim cursor, without a click,
// as gh does in a session that holds the real cursor still.
func (h *handlerState) moveToVimCursor() {
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
		Region:  session.shown,
		Pending: machine.Pending(),
		Current: -1,
	}

	if session.typing {
		frame.Status = ports.VimStatus{
			Mode: "INSERT",
			Kind: ports.VimModeOther,
			Text: "typing into the app, Escape to stop",
		}

		return frame
	}

	if index, ok := text.ContainerAt(cursor); ok && len(text.Containers()) > 1 {
		container := text.Containers()[index]
		frame.Container = container.Bounds

		// The root is the whole screen, which needs no outline.
		if container.Parent > 0 {
			frame.Block = text.Tree()[container.Parent].Bounds
		}
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
	} else if machine.Substituting() {
		// The :s being typed shows what it would change, before Return.
		changes, _ := machine.Preview(0, text.Len())

		for _, change := range changes {
			if r, ok := lineRect(text, change.Start, max(change.End, change.Start+1)); ok {
				frame.Matches = append(frame.Matches, r)
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

	if session.reading && session.preparing {
		frame.Notice = vimPreparingNotice
	}

	frame.Selection = append(frame.Selection, session.flash...)
	frame.Status = vimStatus(session, frame.Current, len(frame.Matches))
	frame.Lens = vimLens(text, cursor)

	// The text read is still where it was before the app scrolled, so the
	// frame moves with the scroll until vim reads the screen again, instead of
	// clearing.
	if session.stale {
		moveVimFrame(&frame, session.staleNear.Sub(h.vimCursorPoint()), session.region)
	}

	return frame
}

// moveVimFrame moves what frame draws over the text by offset, keeping it in
// region, what vim reads. The cursor stops at the region's edge, as the read
// lands it on the edge line, and the rest is cut off at the edge.
func moveVimFrame(frame *ports.VimFrame, offset image.Point, region image.Rectangle) {
	moved := func(rect image.Rectangle) image.Rectangle {
		return rect.Add(offset).Intersect(region)
	}

	if !frame.Cursor.Empty() {
		cursor := frame.Cursor.Add(offset)
		shift := image.Point{
			X: min(max(cursor.Min.X, region.Min.X), region.Max.X-cursor.Dx()) - cursor.Min.X,
			Y: min(max(cursor.Min.Y, region.Min.Y), region.Max.Y-cursor.Dy()) - cursor.Min.Y,
		}
		frame.Cursor = cursor.Add(shift)
	}

	frame.Container = moved(frame.Container)

	var selection []image.Rectangle

	for _, rect := range frame.Selection {
		if rect = moved(rect); !rect.Empty() {
			selection = append(selection, rect)
		}
	}

	frame.Selection = selection

	var matches []image.Rectangle

	current := -1

	for index, rect := range frame.Matches {
		if rect = moved(rect); !rect.Empty() {
			if index == frame.Current {
				current = len(matches)
			}

			matches = append(matches, rect)
		}
	}

	frame.Matches, frame.Current = matches, current

	var labels []ports.VimLabel

	for _, label := range frame.Labels {
		if label.Bounds = moved(label.Bounds); !label.Bounds.Empty() {
			labels = append(labels, label)
		}
	}

	frame.Labels = labels
}

// vimPreparingNotice is what the overlay says while the accurate read waits
// for the vision port to prepare it, which on macOS is Vision compiling its
// models after an install or a macOS update.
const vimPreparingNotice = "Preparing text recognition\n" +
	"Happens once after an install or a macOS update and takes about a minute. " +
	"Moving around works meanwhile."

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

// vimChangeCount says how many places the :s being typed would change, and
// when that is more than one command may change, that Return will refuse it.
func vimChangeCount(changes int) string {
	if changes > vimMaxEdits {
		return fmt.Sprintf("%d to change, more than %d at once", changes, vimMaxEdits)
	}

	return strconv.Itoa(changes) + " to change"
}

// vimCompletion is Tab cycling through the ex commands that start with the
// name typed. at is the match on the line, or len(matches) when the name
// typed is back on it.
type vimCompletion struct {
	typed   string
	matches []string
	at      int
}

// vimCompletionShown is how many matches the status bar lists at once.
const vimCompletionShown = 8

// vimCompletionList lists the page of matches that holds the one on the
// line, which it puts in brackets. A < or > marks more matches before or
// after the page.
func vimCompletionList(completion *vimCompletion) string {
	start := 0
	if completion.at < len(completion.matches) {
		start = completion.at / vimCompletionShown * vimCompletionShown
	}

	end := min(start+vimCompletionShown, len(completion.matches))
	shown := make([]string, 0, vimCompletionShown)

	if start > 0 {
		shown = append(shown, "<")
	}

	for index := start; index < end; index++ {
		name := completion.matches[index]
		if index == completion.at {
			name = "[" + name + "]"
		}

		shown = append(shown, name)
	}

	if end < len(completion.matches) {
		shown = append(shown, ">")
	}

	return strings.Join(shown, " ")
}

// vimStatus is the status bar: the mode, then the command being typed or the
// command line, or the machine's message, and at the right end a macro being
// recorded, where the cursor is among the search matches, and whether a read
// of the screen is still running.
func vimStatus(session *vimSession, current, matches int) ports.VimStatus {
	machine := session.machine
	mode, kind := vimModeName(session)
	status := ports.VimStatus{Mode: mode, Kind: kind}

	switch {
	case session.jump != nil:
		status.Text, status.Typed = "s"+session.jump.query, true
	case machine.Mode() == vim.Cmdline:
		line := machine.Line()
		at := min(max(machine.LineCursor(), 0), len(line))
		status.Text, status.Typed = line[:at]+"▏"+line[at:], true

		if session.completion != nil {
			status.Text += vimStatusGap + vimCompletionList(session.completion)
		}
	case machine.Message() != "":
		status.Text = machine.Message()
	case session.notice != "":
		status.Text = session.notice
	case machine.Shown() != "":
		status.Text = machine.Shown()
	}

	var right []string

	if r := machine.Recording(); r != 0 {
		right = append(right, "recording @"+string(r))
	}

	switch {
	case matches > 0 && machine.Substituting():
		right = append(right, vimChangeCount(matches))
	case matches > 0:
		at := "-"
		if current >= 0 {
			at = strconv.Itoa(current + 1)
		}

		right = append(right, "["+at+"/"+strconv.Itoa(matches)+"]")
	}

	if session.reading || session.stale {
		right = append(right, "reading…")
	}

	status.Right = strings.Join(right, vimStatusGap)

	return status
}

// vimStatusGap separates the parts of the status bar's text.
const vimStatusGap = "   "

// vimModeName names the session's mode for the status bar's chip, and says
// how the chip shows it.
func vimModeName(session *vimSession) (string, ports.VimModeKind) {
	machine := session.machine

	if session.jump != nil {
		return "JUMP", ports.VimModeOther
	}

	if sel, ok := machine.Selection(); ok {
		switch sel.Kind {
		case "V":
			return "V-LINE", ports.VimModeVisual
		case "<C-v>":
			return "V-BLOCK", ports.VimModeVisual
		default:
			return "VISUAL", ports.VimModeVisual
		}
	}

	switch {
	case machine.Mode() == vim.Cmdline:
		return "COMMAND", ports.VimModeOther
	case machine.Pending():
		return "PENDING", ports.VimModeOther
	default:
		return "NORMAL", ports.VimModeNormal
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
