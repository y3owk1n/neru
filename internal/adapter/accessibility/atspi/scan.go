//go:build linux

package atspi

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform"
)

// Finds the frame to hint against: enumerates applications on the AT-SPI bus,
// scores their frames against the compositor's focused app, and picks one.
// This is where the guesswork lives, because AT-SPI exposes no "focused
// window" of its own — see selectFrame for the ranking rules.
//
// isVirtualKeyboardApp reports whether an AT-SPI application is an on-screen
// virtual keyboard, which must never be treated as the focused window.
func isVirtualKeyboardApp(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "plasma-keyboard", "maliit-keyboard", "maliit-server", "squeekboard":
		return true
	default:
		return false
	}
}

// isDesktopShellApp reports whether an AT-SPI application is the KDE desktop
// shell (plasmashell: the panel, taskbar, widgets and desktop background).
// KWin marks plasmashell ACTIVE the instant the pointer moves over the desktop
// — which happens immediately after a hint selection moves the cursor — so it
// would otherwise hijack the active frame on re-activation and yield no app
// hints. It is deprioritised to a last resort rather than skipped entirely so
// its own UI (panel/widgets) can still be hinted when the desktop is genuinely
// the focused surface.
func isDesktopShellApp(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "plasmashell", "org.kde.plasmashell":
		return true
	default:
		return false
	}
}

// isNonTargetSurfaceApp reports whether an AT-SPI application is a system
// surface that is never a valid hint target and must never be picked as the
// focused window — even as a last resort. The XWayland video bridge
// ("xwaylandvideobridge") and the KDE portal consent dialog briefly steal the
// ACTIVE state the moment we inject a cursor move via libei, which on
// re-activation makes findActiveFrame select an empty surface and tears the
// hints overlay down. This mirrors the KWin geometry bridge blocklist in
// internal/adapter/platform/kwin so both code paths ignore the same noise.
func isNonTargetSurfaceApp(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "xwaylandvideobridge",
		"org.freedesktop.impl.portal.desktop.kde":
		return true
	default:
		return false
	}
}

// findActiveFrame locates the focused top-level window. On Wayland the
// compositor's focused app_id is the signal to trust — AT-SPI's ACTIVE state
// lies on wlroots compositors, marking background frames active — so an app_id
// match beats the heuristic. Without one (X11, GNOME, no AT-SPI frame) the
// order is ACTIVE+SHOWING, any ACTIVE, any SHOWING; and when an app_id matches
// no application, that fallback runs only on KWin, where ACTIVE is reliable —
// on wlroots it could pick a background surface, so hints just do not appear.
// focusedAppID/focusedTitle are one snapshot, so frame and stability check
// agree.
func (c *Client) findActiveFrame(
	ctx context.Context,
	conn *dbus.Conn,
	focusedAppID, focusedTitle string,
) (accRef, bool) {
	root := accRef{Name: atspiRegistryDest, Path: atspiRootPath}

	start := time.Now()

	// Fastest path: an event-tracked active window that still matches the
	// compositor's focused toplevel. The background window:activate listener keeps
	// this current, so a match resolves the frame with zero registry scanning. It
	// is validated against the live focus identity, so a stale or missing cache
	// simply falls through to the scan below — never a wrong window.
	if frame, ok := c.activeWindowMatching(focusedAppID, focusedTitle); ok {
		c.logFrameSelection(start, "event-cache", 0, 0)

		return frame, true
	}

	// The title disambiguates multiple windows of the focused application, which
	// share an app_id.
	haveFocused := focusedAppID != ""
	haveFocusedTitle := focusedTitle != ""

	// The AT-SPI ACTIVE state reliably marks the compositor-focused window only
	// on KWin/KDE; on wlroots it is set inconsistently across frames.
	activeStateIdentifiesFocus := platform.DetectLinuxBackend() == platform.BackendWaylandKDE

	apps := c.children(ctx, conn, root)

	// Fast path: when the compositor reports a focused app_id, resolve the window
	// from that one application's frames. The focused app is located with a
	// cancel-on-match name probe (findFocusedApps) that stops the instant a match
	// is seen, so a single wedged background app — common on a desktop with many
	// open windows — can no longer floor the probe at the per-call timeout. Every
	// other app on the bus is skipped. The cross-application ACTIVE/SHOWING
	// fallbacks selectFrame would build from the other apps are never consulted
	// while a focused app_id is present, except the KWin/KDE ACTIVE fallback used
	// when the focused app cannot be uniquely resolved (handled by the full scan
	// below).
	if haveFocused {
		matches := c.findFocusedApps(ctx, conn, apps, focusedAppID)
		if len(matches) > 0 {
			metas := make([]appMeta, len(apps))
			for index := range metas {
				metas[index].skip = true
			}

			for _, index := range matches {
				metas[index] = appMeta{matchesFocused: true}
			}

			scans, scanned := c.scanApps(
				ctx,
				conn,
				apps,
				metas,
				haveFocusedTitle,
				focusedTitle,
				true,
			)
			cand := mergeFrameScan(scans)

			if cand.focusedTitleCount == 1 {
				c.logFrameSelection(start, "focused-title", len(apps), scanned)

				return cand.focusedTitleFrame, true
			}

			if cand.focusedShowingCount == 1 {
				c.logFrameSelection(start, "focused-showing", len(apps), scanned)

				return cand.focusedShowingFrame, true
			}
		}

		if !activeStateIdentifiesFocus {
			// wlroots: the ACTIVE-state fallback is unreliable, so a focused app
			// that cannot be uniquely resolved yields no frame — and there is no
			// reason to scan the rest of the bus.
			c.logFrameSelection(start, "focused-unresolved", len(apps), 0)

			return accRef{}, false
		}

		// KWin/KDE: fall through to the cross-application ACTIVE fallback, which
		// needs every app's frames.
	}

	// Full scan: no focused app_id (X11/GNOME), or KWin/KDE with a focused app
	// that could not be uniquely resolved and needs the cross-app ACTIVE state.
	// This path names every app, so a wedged app can still cost one timeout — but
	// it is reached only without a usable focused app_id, not on the wlroots hot
	// path above.
	names := c.appNames(ctx, conn, apps)

	metas := make([]appMeta, len(apps))

	for index := range apps {
		appName := names[index]

		// Skip surfaces that are never valid hint targets and that steal the
		// ACTIVE state right after a libei cursor move: on-screen virtual
		// keyboards (e.g. the maliit "plasma-keyboard"), the XWayland video
		// bridge, and the portal consent dialog.
		if isVirtualKeyboardApp(appName) || isNonTargetSurfaceApp(appName) {
			metas[index].skip = true

			continue
		}

		metas[index].isShell = isDesktopShellApp(appName)
		metas[index].matchesFocused = haveFocused && !metas[index].isShell &&
			appMatchesFocusedID(appName, focusedAppID)
	}

	scans, scanned := c.scanApps(ctx, conn, apps, metas, haveFocusedTitle, focusedTitle, false)

	cand := mergeFrameScan(scans)
	cand.haveFocused = haveFocused
	cand.activeStateIdentifiesFocus = activeStateIdentifiesFocus

	frame, ok := selectFrame(cand)

	c.logFrameSelection(start, "full-scan", len(apps), scanned)

	return frame, ok
}

// appMeta classifies one AT-SPI application for frame selection: whether it is
// skipped entirely, is the desktop shell, or is the compositor-focused app.
type appMeta struct {
	skip           bool
	isShell        bool
	matchesFocused bool
}

// scannedFrame is one frame of an application captured during the registry scan,
// with the state bits frame selection needs.
type scannedFrame struct {
	ref          accRef
	active       bool
	showing      bool
	titleMatches bool
}

// appFramesScan holds one application's scanned frames plus the classification
// needed to merge them into frameCandidates in registry order.
type appFramesScan struct {
	scanned        bool
	isShell        bool
	matchesFocused bool
	frames         []scannedFrame
}

// appNames reads the accessible Name of every application concurrently. Names
// are latency-bound D-Bus reads that godbus multiplexes over the one connection,
// so overlapping them turns an O(apps) sequential probe into one bounded by the
// slowest single reply. The bounded semaphore caps in-flight calls so a busy
// desktop cannot flood the a11y bus.
func (c *Client) appNames(ctx context.Context, conn *dbus.Conn, apps []accRef) []string {
	names := make([]string, len(apps))

	sem := make(chan struct{}, collectionMaxWorkers)

	var waitGroup sync.WaitGroup

	for index, app := range apps {
		waitGroup.Add(1)

		sem <- struct{}{}

		go func(slot int, app accRef) {
			defer waitGroup.Done()
			defer func() { <-sem }()

			names[slot] = c.name(ctx, conn, app)
		}(index, app)
	}

	waitGroup.Wait()

	return names
}

// findFocusedApps returns the indices of the applications whose name matches the
// compositor's focused app_id. It probes names concurrently and stops the moment
// a match is seen (canceling the outstanding probes), so a single wedged
// background app cannot stall the probe at the per-call timeout — the failure
// mode on a desktop with many open windows, where every frame selection would
// otherwise pay ~atspiCallTimeout waiting on the same unresponsive app. Virtual
// keyboards, non-target surfaces, and the desktop shell are never focus matches
// and are excluded. A no-match probe still waits for every app (there is nothing
// to short-circuit on), but that path yields no frame anyway.
func (c *Client) findFocusedApps(
	ctx context.Context,
	conn *dbus.Conn,
	apps []accRef,
	focusedAppID string,
) []int {
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	matchCh := make(chan int, len(apps))

	go func() {
		sem := make(chan struct{}, collectionMaxWorkers)

		var waitGroup sync.WaitGroup

		for index, app := range apps {
			if probeCtx.Err() != nil {
				break
			}

			waitGroup.Add(1)

			sem <- struct{}{}

			go func(index int, app accRef) {
				defer waitGroup.Done()
				defer func() { <-sem }()

				if probeCtx.Err() != nil {
					return
				}

				name := c.name(probeCtx, conn, app)
				if name == "" ||
					isVirtualKeyboardApp(name) ||
					isNonTargetSurfaceApp(name) ||
					isDesktopShellApp(name) {
					return
				}

				if appMatchesFocusedID(name, focusedAppID) {
					select {
					case matchCh <- index:
					case <-probeCtx.Done():
					}
				}
			}(index, app)
		}

		waitGroup.Wait()
		close(matchCh)
	}()

	var matches []int

	// Block for the first match, then drain any others already delivered before
	// canceling. The common case is a single application (its multiple windows
	// are frames *within* one AT-SPI app, counted later by scanApps), so this
	// returns promptly. Draining still catches other co-active apps that share the
	// app_id, so a genuinely ambiguous focus is reported as ambiguous rather than
	// silently resolved to whichever app answered first. A matching app that only
	// responds after the drain is not counted — acceptable, since that requires
	// two distinct apps sharing one app_id with no title to disambiguate them.
	first, ok := <-matchCh
	if !ok {
		return matches
	}

	matches = append(matches, first)

	for drained := false; !drained; {
		select {
		case index, more := <-matchCh:
			if !more {
				drained = true

				break
			}

			matches = append(matches, index)
		default:
			drained = true
		}
	}

	// Stop probing the rest of the bus so an unresponsive app never gates the
	// result; the background goroutine drains and closes matchCh after the
	// canceled probes return.
	cancel()

	return matches
}

// scanApps reads the frames of the selected applications concurrently, returning
// one appFramesScan per app in registry order (unscanned apps keep a zero value)
// and the total number of frames scanned. When onlyMatching is set, only the
// compositor-focused application(s) are scanned — the fast path — otherwise every
// non-skipped app is. Each goroutine writes its own slot, so no lock is needed.
func (c *Client) scanApps(
	ctx context.Context,
	conn *dbus.Conn,
	apps []accRef,
	metas []appMeta,
	haveFocusedTitle bool,
	focusedTitle string,
	onlyMatching bool,
) ([]appFramesScan, int) {
	results := make([]appFramesScan, len(apps))

	sem := make(chan struct{}, collectionMaxWorkers)

	var waitGroup sync.WaitGroup

	for index := range apps {
		meta := metas[index]
		if meta.skip || (onlyMatching && !meta.matchesFocused) {
			continue
		}

		waitGroup.Add(1)

		sem <- struct{}{}

		go func(slot int, meta appMeta) {
			defer waitGroup.Done()
			defer func() { <-sem }()

			results[slot] = c.scanOneApp(
				ctx,
				conn,
				apps[slot],
				meta,
				haveFocusedTitle,
				focusedTitle,
			)
		}(index, meta)
	}

	waitGroup.Wait()

	framesScanned := 0
	for i := range results {
		framesScanned += len(results[i].frames)
	}

	return results, framesScanned
}

// scanOneApp reads an application's window/dialog frames and their ACTIVE/SHOWING
// state (one GetState per frame), plus the title of the focused app's showing
// frames so a sibling window can be disambiguated. It is safe for concurrent use:
// it only issues reads over the shared connection, which godbus multiplexes.
func (c *Client) scanOneApp(
	ctx context.Context,
	conn *dbus.Conn,
	app accRef,
	meta appMeta,
	haveFocusedTitle bool,
	focusedTitle string,
) appFramesScan {
	res := appFramesScan{scanned: true, isShell: meta.isShell, matchesFocused: meta.matchesFocused}

	for _, frame := range c.children(ctx, conn, app) {
		role := c.roleName(ctx, conn, frame)
		if !isWindowRole(role) {
			continue
		}

		active, showing := c.frameStates(ctx, conn, frame)

		scanned := scannedFrame{ref: frame, active: active, showing: showing}

		// Only the focused app's showing frames need a title read, and only to
		// disambiguate its sibling windows.
		if meta.matchesFocused && showing && haveFocusedTitle {
			scanned.titleMatches = titleMatchesFocused(c.name(ctx, conn, frame), focusedTitle)
		}

		res.frames = append(res.frames, scanned)
	}

	return res
}

// mergeFrameScan folds the per-app scans into frameCandidates in registry order,
// reproducing the original sequential selection exactly: the desktop shell is
// held aside, the focused app's showing/title-matching windows are counted, and
// the first ACTIVE/SHOWING frames become the cross-application fallbacks.
func mergeFrameScan(scans []appFramesScan) frameCandidates {
	var cand frameCandidates

	for _, scan := range scans {
		if !scan.scanned {
			continue
		}

		for _, frame := range scan.frames {
			// The desktop shell never wins the active-frame race; it is kept aside
			// and only used if no real application frame is found.
			if scan.isShell {
				if frame.showing && !cand.haveShell {
					cand.shellShowing = frame.ref
					cand.haveShell = true
				}

				continue
			}

			// Count the focused application's showing windows and how many match
			// the focused toplevel's title, so selectFrame can tell a unique
			// identification from an ambiguous one.
			if scan.matchesFocused && frame.showing {
				cand.focusedShowingCount++

				if cand.focusedShowingCount == 1 {
					cand.focusedShowingFrame = frame.ref
				}

				if frame.titleMatches {
					cand.focusedTitleCount++

					if cand.focusedTitleCount == 1 {
						cand.focusedTitleFrame = frame.ref
					}
				}
			}

			if frame.active && frame.showing && !cand.haveActiveShowing {
				cand.activeShowing = frame.ref
				cand.haveActiveShowing = true
			}

			if frame.active && !cand.haveActiveAny {
				cand.activeAny = frame.ref
				cand.haveActiveAny = true
			}

			if frame.showing && !cand.haveShowingAny {
				cand.showingAny = frame.ref
				cand.haveShowingAny = true
			}
		}
	}

	return cand
}

// logFrameSelection records how frame selection resolved and how much of the bus
// it had to scan, so a slow selection on a busy desktop is visible in the logs.
func (c *Client) logFrameSelection(start time.Time, path string, apps, framesScanned int) {
	c.logger.Debug("AT-SPI frame selection complete",
		zap.String("path", path),
		zap.Int("apps", apps),
		zap.Int("framesScanned", framesScanned),
		zap.Duration("elapsed", time.Since(start)))
}

// frameCandidates holds the frames found during a findActiveFrame walk, ranked
// by how reliably each identifies the focused window, plus the compositor
// signals needed to choose between them.
type frameCandidates struct {
	// Frames of the compositor-focused application, with the counts needed to
	// tell a unique identification from an ambiguous one.
	focusedTitleFrame   accRef // first window whose title matched the focused toplevel
	focusedTitleCount   int    // windows of the focused app matching that title
	focusedShowingFrame accRef // first showing window of the focused app
	focusedShowingCount int    // showing windows of the focused app

	// Cross-application ACTIVE/SHOWING fallbacks (reliable only on KDE).
	activeShowing     accRef
	haveActiveShowing bool
	activeAny         accRef
	haveActiveAny     bool
	showingAny        accRef
	haveShowingAny    bool

	// Desktop shell, used only as a last resort when nothing is focused.
	shellShowing accRef
	haveShell    bool

	// haveFocused is true when the compositor reported a focused app_id.
	haveFocused bool
	// activeStateIdentifiesFocus is true only where the AT-SPI ACTIVE state
	// reliably marks the focused window (KWin/KDE).
	activeStateIdentifiesFocus bool
}

// selectFrame chooses the focused frame from the ranked candidates. It is the
// pure decision half of findActiveFrame, split out so the ordering can be
// unit-tested without a live AT-SPI bus.
func selectFrame(cand frameCandidates) (accRef, bool) {
	switch {
	case cand.focusedTitleCount == 1:
		// Exactly one window of the focused app carries the focused toplevel's
		// title — an unambiguous match (works on any compositor).
		return cand.focusedTitleFrame, true
	case cand.focusedShowingCount == 1:
		// The focused app has a single showing window, so it is unambiguous even
		// without a title match.
		return cand.focusedShowingFrame, true
	case cand.haveFocused:
		// A focused app_id was reported but its window could not be uniquely
		// identified — no AT-SPI frame matched (name differs from the app_id, or
		// it exposes no AT-SPI), or several sibling windows share/lack the title.
		// On KWin/KDE the ACTIVE state still marks the focused window, so an
		// ACTIVE frame is it (covers name-mismatched apps such as GTK "Files" vs
		// org.gnome.Nautilus and duplicate-title siblings). On wlroots ACTIVE is
		// unreliable, so return nothing rather than guess a sibling or a
		// background window.
		if cand.activeStateIdentifiesFocus && cand.haveActiveShowing {
			return cand.activeShowing, true
		}

		if cand.activeStateIdentifiesFocus && cand.haveActiveAny {
			return cand.activeAny, true
		}

		return accRef{}, false
	case cand.haveActiveShowing:
		return cand.activeShowing, true
	case cand.haveActiveAny:
		return cand.activeAny, true
	case cand.haveShowingAny:
		return cand.showingAny, true
	case cand.haveShell:
		return cand.shellShowing, true
	default:
		return accRef{}, false
	}
}

// titleMatchesFocused reports whether an AT-SPI frame title equals the
// compositor's focused toplevel title. Both derive from the window's
// xdg_toplevel.title, so a whitespace-trimmed exact comparison disambiguates
// windows of the same application. Empty titles never match.
func titleMatchesFocused(frameTitle, focusedTitle string) bool {
	frameTitle = strings.TrimSpace(frameTitle)

	return frameTitle != "" && frameTitle == strings.TrimSpace(focusedTitle)
}

// appMatchesFocusedID reports whether an AT-SPI application name corresponds to
// the compositor's focused app_id. The two vocabularies differ: AT-SPI reports
// a display name ("Firefox") while wlr-foreign-toplevel reports an app_id that
// is often reverse-DNS ("org.mozilla.firefox") or a bare id ("helium"). The
// match is case-insensitive and also compares the last dot-segment of either
// side, so "Firefox" matches "org.mozilla.firefox" and "Helium" matches
// "helium".
func appMatchesFocusedID(atspiName, appID string) bool {
	name := strings.ToLower(strings.TrimSpace(atspiName))
	focusedID := strings.ToLower(strings.TrimSpace(appID))

	if name == "" || focusedID == "" {
		return false
	}

	if name == focusedID {
		return true
	}

	if seg := lastDotSegment(focusedID); seg != "" && seg == name {
		return true
	}

	if seg := lastDotSegment(name); seg != "" && seg == focusedID {
		return true
	}

	return false
}

// lastDotSegment returns the substring after the final '.', or "" when there is
// no interior dot (nothing after the dot, or no dot at all).
func lastDotSegment(value string) string {
	i := strings.LastIndex(value, ".")
	if i < 0 || i == len(value)-1 {
		return ""
	}

	return value[i+1:]
}
