//go:build linux

package atspi

import (
	"image"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/kwin"
	"github.com/y3owk1n/neru/internal/derrors"
)

// KDE window-origin source. KWin exposes no CLI the way the wlroots family
// does, so the geometry comes from the KWin script
// [github.com/y3owk1n/neru/internal/adapter/platform/kwin] installs — the same
// bridge SystemPort.FocusedWindowBounds reads, because it is the same fact.
// This file is only the AT-SPI half of it: turning the cached rectangle into
// the origin that offsets window-relative element coordinates.

// kwinGeometry is the part of the bridge this source uses. It is named here so
// the rules for accepting or rejecting a cached window can be tested without a
// KWin — which is the only way they can be tested at all, since the bridge is
// process-wide by design and a test that pushed into it would be pushing into
// every other test's cache too.
type kwinGeometry interface {
	EnsureStarted()
	Focused() (kwin.Window, bool, error)
}

type kwinOriginSource struct {
	logger   *zap.Logger
	geometry kwinGeometry
}

func newKWinOriginSource(logger *zap.Logger) *kwinOriginSource {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &kwinOriginSource{
		logger:   logger.Named("accessibility.kwin"),
		geometry: kwin.Shared(logger),
	}
}

func (s *kwinOriginSource) start() { s.geometry.EnsureStarted() }

// originFor returns the cached origin only when the cached window is the window
// the given frame describes.
//
// The cache is fed by KWin, and the frame was selected from AT-SPI, so nothing
// guarantees the two are the same window: if KWin ignored a transient surface
// that became the active AT-SPI frame, or a transition reached one of them
// first, the cached origin belongs to a different window and offsetting by it
// would land every hint at that window's screen position. Two checks stand
// between the cache and that outcome, in order of how much they can tell apart:
//
//   - Identity. KWin reports which window its rectangle belongs to, and the
//     frame carries the compositor's focused-toplevel identity it was selected
//     with. Both descriptions come from KWin, so a disagreement is conclusive:
//     the rectangle is some other window's.
//   - Size, within windowOriginSizeTolerance. It is the only check available
//     when the identity is not — and it cannot separate two windows that happen
//     to be the same size (#972), which is why it is no longer the only one.
//
// Neither check refuses when it has nothing to compare. An identity the
// compositor never reported is not a mismatch, and treating it as one would
// leave every hint on such a session unoffset — a far larger failure than the
// one being closed here.
//
// A bridge that could not be installed is reported rather than folded into that
// same answer. The degradation is identical — an unoffset hint either way — but
// one of them is a KDE session that will never place a hint correctly until
// something is fixed, and telling a person that is the whole reason this source
// carries a reason at all.
//
// It is reported in the same words the focused-window arm uses for the same
// failure, because it is the same bridge: CodeNotSupported, because what a
// person has to do about it is not check a permission but install a script that
// is missing. One geometry source answering one way here and another there is
// what having one geometry source is for.
func (s *kwinOriginSource) originFor(frame windowFrame) (image.Point, bool, error) {
	// Asked on every activation rather than once at construction, for the same
	// reason the focused-window arm asks on every call: the bridge can lose its
	// install after having one — KWin restarting takes the script with it — and
	// a source that only ever tried at startup would spend the rest of the
	// session placing hints window-relative. It is free once installed and never
	// blocks, so the mode handler's lock is not held on anything.
	s.geometry.EnsureStarted()

	window, cached, err := s.geometry.Focused()
	if err != nil {
		return image.Point{}, false, derrors.Wrap(
			err,
			derrors.CodeNotSupported,
			"the KWin focused-window geometry script is not installed",
		)
	}

	if !cached {
		return image.Point{}, false, nil
	}

	if !s.isFrameWindow(window, frame) {
		return image.Point{}, false, nil
	}

	rect := window.Rect
	if absInt(rect.Dx()-frame.Width) > windowOriginSizeTolerance ||
		absInt(rect.Dy()-frame.Height) > windowOriginSizeTolerance {
		s.logger.Debug("KWin origin rejected: cached size does not match AT-SPI frame",
			zap.Int("cachedW", rect.Dx()), zap.Int("cachedH", rect.Dy()),
			zap.Int("frameW", frame.Width), zap.Int("frameH", frame.Height))

		return image.Point{}, false, nil
	}

	return rect.Min, true, nil
}

// isFrameWindow reports whether the window KWin cached is the one the frame was
// selected for, as far as their two identities can say.
//
// Both identities originate in KWin — the cached one is a script reading
// resourceClass, resourceName and caption; the frame's is the same compositor's
// wlr-foreign-toplevel-management app_id and title — so where both sides have
// something to say, a disagreement means two different windows.
//
// Where either side is silent the answer is yes. X11 and GNOME have no
// foreign-toplevel protocol to read an identity from at all, and a KWin version
// that reports no identifiers is not thereby reporting a different window.
// Refusing on a missing identity would turn a check that closes a rare
// misplacement into one that unoffsets every hint on a whole session.
//
// The same asymmetry governs how tolerant each comparison is. A false accept
// costs at most what this check cost before it existed; a false reject unoffsets
// hints that were placed correctly. So each comparison is written to be sure
// before it refuses.
func (s *kwinOriginSource) isFrameWindow(window kwin.Window, frame windowFrame) bool {
	if !kwinAppMatches(window, frame.FocusedAppID) {
		s.logger.Debug("KWin origin rejected: cached window is a different application")

		return false
	}

	if !kwinTitleMatches(window.Title, frame.FocusedTitle) {
		s.logger.Debug("KWin origin rejected: cached window is a different window " +
			"of the focused application")

		return false
	}

	return true
}

// kwinAppMatches reports whether either of KWin's two identifiers for the cached
// window names the compositor's focused application.
//
// Either is allowed to be the match because the two disagree for some windows —
// an XWayland Firefox is resourceClass "firefox" and resourceName "navigator" —
// and which of them the foreign-toplevel app_id was built from is KWin's
// business, not something to be guessed at here. Each comparison is the tolerant
// one AT-SPI frame selection already uses, since the same application is spelled
// "konsole" in one vocabulary and "org.kde.konsole" in another.
func kwinAppMatches(window kwin.Window, appID string) bool {
	if appID == "" || (window.Class == "" && window.Name == "") {
		return true
	}

	return appMatchesFocusedID(window.Class, appID) || appMatchesFocusedID(window.Name, appID)
}

// kwinTitleMatches reports whether KWin's caption and the compositor's focused
// title describe one window.
//
// A prefix counts as a match, in either direction, because KWin's caption is the
// window's title plus a suffix it adds itself — a window shortcut, a hostname, a
// "<2>" when two windows share a title — and nothing here can know whether the
// foreign-toplevel title was built before or after that suffix was appended. An
// exact comparison would therefore reject every activation of a window that has
// one, permanently unoffsetting its hints.
//
// What survives is what this comparison is for: two windows of one application
// showing different documents have titles that are not prefixes of each other,
// and that is the case size alone cannot separate (#972). Two windows whose
// titles agree up to a "<2>" are not separated — but neither is anything else
// about them, which frame selection already says of identical titles.
func kwinTitleMatches(caption, focusedTitle string) bool {
	caption = strings.TrimSpace(caption)
	focused := strings.TrimSpace(focusedTitle)

	if caption == "" || focused == "" {
		return true
	}

	return strings.HasPrefix(caption, focused) || strings.HasPrefix(focused, caption)
}
