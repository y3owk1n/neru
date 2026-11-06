//go:build linux

package atspi

import (
	"image"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/kwin"
	"github.com/y3owk1n/neru/internal/derrors"
)

// KDE window-origin source. KWin has no CLI, so the geometry comes from the
// script [github.com/y3owk1n/neru/internal/adapter/platform/kwin] installs,
// the same bridge SystemPort.FocusedWindowBounds reads. This file turns its
// cached rectangle into an origin.

// kwinGeometry is the part of the bridge this source uses, an interface so the
// acceptance rules can be tested without KWin. The bridge is process-wide, so
// a test pushing into it would push into every other test's cache.
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

// originFor returns the cached origin only when the cached window is the one
// frame describes.
//
// The cache comes from KWin and the frame from AT-SPI, so they can differ (a
// transient surface KWin ignored, a transition one side saw first), and that
// origin would move every hint to the other window. Two checks guard it:
//
//   - Identity. KWin says whose rectangle it is, and the frame carries the
//     focused-toplevel identity it was selected with. Both come from KWin, so
//     a disagreement means another window.
//   - Size, within windowOriginSizeTolerance. It is all there is without an
//     identity, and it cannot separate two same-sized windows (#972).
//
// Neither refuses with nothing to compare, since that would unoffset every
// hint on such a session.
//
// A bridge that could not be installed is reported, in the same words and with
// the same CodeNotSupported as the focused-window arm, because that KDE
// session places no hint correctly until the missing script is installed.
func (s *kwinOriginSource) originFor(frame windowFrame) (image.Point, bool, error) {
	// Asked on every activation, since a KWin restart takes the script with it.
	// It is free once installed and never blocks, so the mode handler's lock is
	// not held on anything.
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
			zap.Int("cached_w", rect.Dx()), zap.Int("cached_h", rect.Dy()),
			zap.Int("frame_w", frame.Width), zap.Int("frame_h", frame.Height))

		return image.Point{}, false, nil
	}

	return rect.Min, true, nil
}

// isFrameWindow reports whether the window KWin cached is the one the frame
// was selected for, as far as their identities can say. Both come from KWin
// (the script's resourceClass, resourceName and caption, and the compositor's
// foreign-toplevel app_id and title), so where both speak, a disagreement
// means two windows. Where either is silent the answer is yes, since X11 and
// GNOME have no such protocol. A false accept costs what no check did, while
// a false reject unoffsets correct hints, so each comparison is sure before
// it refuses.
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

// kwinAppMatches reports whether either of KWin's identifiers for the cached
// window names the focused application. Either may match because they differ
// for some windows (XWayland Firefox is class "firefox", name "navigator").
// Each comparison is frame selection's tolerant one ("konsole" vs
// "org.kde.konsole").
func kwinAppMatches(window kwin.Window, appID string) bool {
	if appID == "" || (window.Class == "" && window.Name == "") {
		return true
	}

	return appMatchesFocusedID(window.Class, appID) || appMatchesFocusedID(window.Name, appID)
}

// kwinTitleMatches reports whether KWin's caption and the focused title
// describe one window. A prefix either way counts, because KWin appends its
// own suffix (a shortcut, a hostname, "<2>" for a shared title) and the
// foreign-toplevel title may predate it. Exact matching would unoffset such a
// window for good. It still catches two windows of one app showing different
// documents, which size cannot (#972).
func kwinTitleMatches(caption, focusedTitle string) bool {
	caption = strings.TrimSpace(caption)
	focused := strings.TrimSpace(focusedTitle)

	if caption == "" || focused == "" {
		return true
	}

	return strings.HasPrefix(caption, focused) || strings.HasPrefix(focused, caption)
}
