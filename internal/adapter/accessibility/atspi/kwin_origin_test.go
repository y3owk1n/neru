//go:build linux

package atspi

import (
	"errors"
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/kwin"
	"github.com/y3owk1n/neru/internal/derrors"
)

const (
	// titleKonsoleBash is the caption of the window the tests below cache.
	titleKonsoleBash = "~ : bash"

	// nameFirefoxX11 is Firefox's resourceName, which is not its resourceClass
	// — the disagreement that makes matching either identifier necessary.
	nameFirefoxX11 = "navigator"
)

// errTestKWinAbsent stands in for whatever kept the bridge from installing. The
// origin source only has to carry it back, so the wording is deliberately not
// the bridge's own: what is pinned is that any reason survives the wrap.
var errTestKWinAbsent = errors.New("the KWin geometry script could not be loaded")

// fakeKWinGeometry stands in for the process-wide bridge, so what the KWin
// origin source does with a cached window can be decided in a test rather than
// by whatever a KDE session happened to be doing.
type fakeKWinGeometry struct {
	window kwin.Window
	cached bool
	err    error
}

func (f fakeKWinGeometry) EnsureStarted() {}

func (f fakeKWinGeometry) Focused() (kwin.Window, bool, error) {
	return f.window, f.cached, f.err
}

func kwinSourceWith(geometry kwinGeometry) *kwinOriginSource {
	return &kwinOriginSource{logger: zap.NewNop(), geometry: geometry}
}

// cachedKonsole is a window KWin reported, sized to match the frame every test
// below asks about, so size is never what decides the answer.
func cachedKonsole(title string) kwin.Window {
	return kwin.Window{
		Rect:  image.Rect(100, 50, 900, 650),
		Class: appKonsole,
		Title: title,
	}
}

// konsoleFrame is the AT-SPI frame those tests are walking: the same size, and
// the compositor identity it was selected with.
func konsoleFrame(appID, title string) windowFrame {
	return windowFrame{Width: 800, Height: 600, FocusedAppID: appID, FocusedTitle: title}
}

// TestKWinOriginSource_OriginForOffsetsByTheMatchingWindow is the ordinary case:
// the cache and the frame describe one window, so hints are placed at its screen
// position.
func TestKWinOriginSource_OriginForOffsetsByTheMatchingWindow(t *testing.T) {
	source := kwinSourceWith(fakeKWinGeometry{
		window: cachedKonsole(titleKonsoleBash),
		cached: true,
	})

	origin, ok, err := source.originFor(konsoleFrame(appKonsole, titleKonsoleBash))
	if !ok || err != nil {
		t.Fatalf("originFor() = (%v, %v, %v), want the cached origin", origin, ok, err)
	}

	if want := image.Pt(100, 50); origin != want {
		t.Errorf("originFor() = %v, want %v", origin, want)
	}
}

// TestKWinOriginSource_OriginForRejectsAnotherApplication covers the case size
// alone already caught only by luck. Two windows of different applications can
// be the same size, and offsetting by the wrong one puts every hint in this
// window at the other window's screen position.
func TestKWinOriginSource_OriginForRejectsAnotherApplication(t *testing.T) {
	source := kwinSourceWith(fakeKWinGeometry{
		window: cachedKonsole(titleKonsoleBash),
		cached: true,
	})

	origin, ok, err := source.originFor(konsoleFrame("kate", "untitled — Kate"))
	if ok || err != nil {
		t.Fatalf("originFor() = (%v, %v, %v), want no origin and no error — the "+
			"cached rectangle belongs to another window, and hints fall back to "+
			"window-relative rather than to the wrong screen position", origin, ok, err)
	}
}

// TestKWinOriginSource_OriginForRejectsASiblingWindow is #972 exactly: two
// windows of the same application, the same size, so the size check passes and
// the origin is the other window's. Only the title separates them.
func TestKWinOriginSource_OriginForRejectsASiblingWindow(t *testing.T) {
	source := kwinSourceWith(fakeKWinGeometry{
		window: cachedKonsole(titleKonsoleBash),
		cached: true,
	})

	origin, ok, err := source.originFor(konsoleFrame(appKonsole, "/var/log : less"))
	if ok || err != nil {
		t.Fatalf("originFor() = (%v, %v, %v), want no origin — two same-sized "+
			"windows of one application are what the size check cannot tell apart",
			origin, ok, err)
	}
}

// TestKWinOriginSource_OriginForToleratesIdentifierSpelling keeps the identity
// check from rejecting windows that match.
//
// The two sides read the same application through different vocabularies —
// KWin's resourceClass and resourceName, and the foreign-toplevel app_id built
// from one of them — which spell it differently often enough that an exact
// comparison against either alone would refuse working sessions. A false reject
// costs unoffset hints on every activation of that window; a false accept costs
// no more than having no check.
func TestKWinOriginSource_OriginForToleratesIdentifierSpelling(t *testing.T) {
	cases := []struct {
		name  string
		class string
		app   string
		appID string
	}{
		{"identical", appKonsole, appKonsole, appKonsole},
		{"different case", "Konsole", appKonsole, appKonsole},
		{"reverse-dns on the compositor side", appKonsole, appKonsole, appKonsoleDotted},
		{"reverse-dns on the script side", appKonsoleDotted, appKonsole, appKonsole},
		// An XWayland window's two identifiers disagree, and the app_id is
		// built from whichever one KWin chose. Matching either is what keeps
		// the choice from being something this check has to guess.
		{"app_id built from resourceClass", appFirefox, nameFirefoxX11, appFirefox},
		{"app_id built from resourceName", appFirefox, nameFirefoxX11, nameFirefoxX11},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			window := cachedKonsole(titleKonsoleBash)
			window.Class, window.Name = testCase.class, testCase.app

			source := kwinSourceWith(fakeKWinGeometry{window: window, cached: true})

			_, ok, err := source.originFor(konsoleFrame(testCase.appID, titleKonsoleBash))
			if !ok || err != nil {
				t.Errorf("originFor() = (%v, %v) for class %q / name %q against "+
					"app_id %q, want the origin",
					ok, err, testCase.class, testCase.app, testCase.appID)
			}
		})
	}
}

// TestKWinTitleMatches pins where the title comparison draws its line.
//
// KWin's caption is the window's title plus a suffix it adds itself — a window
// shortcut, a hostname, a "<2>" when two windows share a title — and nothing
// here knows whether the foreign-toplevel title was built before or after that
// suffix. So a prefix in either direction is a match: an exact comparison would
// unoffset every hint in a window that has a shortcut assigned, forever, to
// close a case it was never the only defense against.
func TestKWinTitleMatches(t *testing.T) {
	cases := []struct {
		name    string
		caption string
		focused string
		want    bool
	}{
		{"identical", titleKonsoleBash, titleKonsoleBash, true},
		{"whitespace only", "  ~ : bash ", titleKonsoleBash, true},
		{"kwin appended a shortcut suffix", "Kate {Ctrl+1}", "Kate", true},
		{"the compositor carried the suffix", "Kate", "Kate {Ctrl+1}", true},
		{"different documents", "notes.md — Kate", "main.go — Kate", false},
		{"different applications entirely", titleKonsoleBash, "untitled — Kate", false},
		{"no caption to compare", "", titleKonsoleBash, true},
		{"no focused title to compare", titleKonsoleBash, "", true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := kwinTitleMatches(testCase.caption, testCase.focused); got != testCase.want {
				t.Errorf("kwinTitleMatches(%q, %q) = %v, want %v",
					testCase.caption, testCase.focused, got, testCase.want)
			}
		})
	}
}

// TestKWinOriginSource_OriginForAcceptsAnAbsentIdentity is the half that keeps
// this check from being worse than what it replaces.
//
// A missing identity is not a mismatch: X11 and GNOME expose no
// foreign-toplevel protocol to read one from, and a KWin that reports no class
// or caption has not thereby reported a different window. Refusing here would
// unoffset every hint on a whole session to close a rare misplacement.
func TestKWinOriginSource_OriginForAcceptsAnAbsentIdentity(t *testing.T) {
	cases := []struct {
		name   string
		window kwin.Window
		frame  windowFrame
	}{
		{
			name:   "the compositor reported no identity",
			window: cachedKonsole(titleKonsoleBash),
			frame:  konsoleFrame("", ""),
		},
		{
			name:   "the script reported no identifiers or caption",
			window: kwin.Window{Rect: image.Rect(100, 50, 900, 650)},
			frame:  konsoleFrame(appKonsole, titleKonsoleBash),
		},
		{
			name:   "only the titles are missing",
			window: cachedKonsole(""),
			frame:  konsoleFrame(appKonsole, ""),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			source := kwinSourceWith(fakeKWinGeometry{window: testCase.window, cached: true})

			_, ok, err := source.originFor(testCase.frame)
			if !ok || err != nil {
				t.Errorf("originFor() = (%v, %v), want the origin — an identity "+
					"nobody reported is not a mismatch", ok, err)
			}
		})
	}
}

// TestKWinOriginSource_OriginForStillRejectsASizeMismatch keeps the older check
// in place. It is the only one available when there is no identity to compare,
// and a window resized between the AT-SPI read and the cache is a mismatch the
// identities agree about.
func TestKWinOriginSource_OriginForStillRejectsASizeMismatch(t *testing.T) {
	window := cachedKonsole(titleKonsoleBash)
	window.Rect = image.Rect(100, 50, 1300, 950)

	source := kwinSourceWith(fakeKWinGeometry{window: window, cached: true})

	origin, ok, err := source.originFor(konsoleFrame(appKonsole, titleKonsoleBash))
	if ok || err != nil {
		t.Fatalf("originFor() = (%v, %v, %v), want no origin", origin, ok, err)
	}
}

// TestKWinOriginSource_OriginForReportsNothingCachedAsAnAnswer covers the state
// a clear leaves behind. Nothing is focused, which is the compositor answering,
// so hints go window-relative without anyone being told something is broken.
func TestKWinOriginSource_OriginForReportsNothingCachedAsAnAnswer(t *testing.T) {
	source := kwinSourceWith(fakeKWinGeometry{})

	origin, ok, err := source.originFor(konsoleFrame(appKonsole, titleKonsoleBash))
	if ok || err != nil {
		t.Fatalf("originFor() = (%v, %v, %v), want (empty, false, nil)", origin, ok, err)
	}
}

// TestKWinOriginSource_OriginForReportsAFailedInstall keeps a broken bridge
// distinguishable from an idle one. Both leave hints window-relative, and only
// one is something a person can fix — so the reason has to reach the caller
// rather than be folded into "no origin".
func TestKWinOriginSource_OriginForReportsAFailedInstall(t *testing.T) {
	source := kwinSourceWith(fakeKWinGeometry{err: errTestKWinAbsent})

	_, ok, err := source.originFor(konsoleFrame(appKonsole, titleKonsoleBash))
	if ok {
		t.Fatal("originFor() reported an origin from a bridge that never installed")
	}

	if !errors.Is(err, errTestKWinAbsent) {
		t.Errorf("originFor() error = %v, want it to carry %v", err, errTestKWinAbsent)
	}

	if !derrors.IsNotSupported(err) {
		t.Errorf("originFor() code = %v, want %v — what a person has to do about "+
			"this is not check a permission", derrors.GetCode(err), derrors.CodeNotSupported)
	}
}
