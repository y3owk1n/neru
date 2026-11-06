//go:build linux

package atspi

import (
	"context"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform"
)

// Frame selection. AT-SPI exposes no focused window of its own, so this scores
// each application's frames against the compositor's focused app. See
// selectFrame for the ranking.
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
// shell (panel, taskbar, widgets, background). KWin marks it ACTIVE as soon
// as the pointer crosses the desktop, as it does right after a hint moves the
// cursor. It is a last resort rather than skipped, so its own UI stays
// hintable when the desktop really is focused.
func isDesktopShellApp(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "plasmashell", "org.kde.plasmashell":
		return true
	default:
		return false
	}
}

// isNonTargetSurfaceApp reports whether an AT-SPI application is a system
// surface that must never be picked, even as a last resort. The XWayland
// video bridge and the KDE portal consent dialog briefly take ACTIVE when
// libei moves the cursor. Keep it in step with the KWin geometry bridge
// blocklist in internal/adapter/platform/kwin.
func isNonTargetSurfaceApp(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "xwaylandvideobridge",
		"org.freedesktop.impl.portal.desktop.kde":
		return true
	default:
		return false
	}
}

// findActiveFrame locates the focused top-level window. The focused app_id
// (the Wayland toplevel, or the WM_CLASS of _NET_ACTIVE_WINDOW on X11) beats
// AT-SPI's ACTIVE state, which marks background frames active on wlroots and
// on Chromium/Electron under X11. Without an app_id the order is
// ACTIVE+SHOWING, any ACTIVE, any SHOWING. When an app_id matches no
// application that fallback runs on KWin, where ACTIVE is reliable, and on
// X11. On wlroots hints just do not appear. focusedAppID/focusedTitle are one
// snapshot, so frame and stability check agree.
func (c *Client) findActiveFrame(
	ctx context.Context,
	conn *dbus.Conn,
	focusedAppID, focusedTitle string,
) (accRef, bool) {
	root := accRef{Name: atspiRegistryDest, Path: atspiRootPath}

	start := time.Now()

	// Fastest path: the window:activate cache, validated against the live focus
	// identity so a stale or missing entry falls through to the scan and never
	// resolves to the wrong window.
	if frame, ok := c.activeWindowMatching(focusedAppID, focusedTitle); ok {
		c.logFrameSelection(start, "event-cache", 0, 0)

		return frame, true
	}

	// The title disambiguates multiple windows of the focused application, which
	// share an app_id.
	haveFocused := focusedAppID != ""
	haveFocusedTitle := focusedTitle != ""

	backend := platform.DetectLinuxBackend()

	// The AT-SPI ACTIVE state reliably marks the compositor-focused window only
	// on KWin/KDE. On wlroots it is set inconsistently across frames.
	activeStateIdentifiesFocus := backend == platform.BackendWaylandKDE

	// On X11 ACTIVE holds within one application but not across the bus, because
	// Chromium/Electron frames advertise ACTIVE whatever _NET_ACTIVE_WINDOW says
	// (#1644). So a single ACTIVE frame may pick among the focused app's siblings.
	focusedActiveDisambiguates := backend == platform.BackendX11

	apps := c.children(ctx, conn, root)

	// Fast path: resolve the window from the focused application's frames alone.
	// findFocusedApps cancels on the first name match, so a wedged background app
	// cannot floor the probe at the per-call timeout. The cross-application
	// fallbacks are skipped, except KWin's ACTIVE fallback when the focused app
	// cannot be uniquely resolved (the full scan below).
	if haveFocused {
		matches, names := c.findFocusedApps(ctx, conn, apps, focusedAppID)

		c.logger.Debug("AT-SPI focused app probe",
			zap.Int("apps", len(apps)),
			zap.Int("matches", len(matches)),
			zap.Bool("have_title", haveFocusedTitle))

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

			c.logger.Debug("AT-SPI focused app frames",
				zap.Int("showing", cand.focusedShowingCount),
				zap.Int("title_matches", cand.focusedTitleCount),
				zap.Int("active", cand.focusedActiveCount))

			if cand.focusedTitleCount == 1 {
				c.logFrameSelection(start, "focused-title", len(apps), scanned)

				return cand.focusedTitleFrame, true
			}

			if cand.focusedShowingCount == 1 {
				c.logFrameSelection(start, "focused-showing", len(apps), scanned)

				return cand.focusedShowingFrame, true
			}

			if focusedActiveDisambiguates {
				cand.haveFocused = true
				cand.focusedActiveDisambiguates = true

				if frame, ok := selectFrame(cand); ok {
					c.logFrameSelection(start, "focused-active", len(apps), scanned)

					return frame, true
				}
			}
		}

		if len(matches) == 0 && haveFocusedTitle {
			if frame, path, scanned, ok := c.findFocusedFrameByTitle(
				ctx, conn, apps, names, focusedAppID, focusedTitle); ok {
				c.logFrameSelection(start, path, len(apps), scanned)

				return frame, true
			}
		}

		switch {
		case activeStateIdentifiesFocus:
			// KWin/KDE: fall through to the cross-application ACTIVE fallback,
			// which needs every app's frames.
		case focusedActiveDisambiguates:
			// X11: no AT-SPI application answered to the WM_CLASS, so the focus
			// identity cannot narrow the scan. Drop it and run the cross-app heuristic.
			haveFocused = false
		default:
			// wlroots: the ACTIVE-state fallback is unreliable, so a focused app
			// that cannot be uniquely resolved yields no frame, and there is no
			// reason to scan the rest of the bus.
			c.logFrameSelection(start, "focused-unresolved", len(apps), 0)

			return accRef{}, false
		}
	}

	// Full scan: no focused app_id (GNOME without the extension), an X11 focus no
	// AT-SPI app answers to, or KWin with an unresolved focused app. Naming every
	// app means a wedged one can cost a timeout, but this is off the wlroots hot
	// path.
	names := c.appNames(ctx, conn, apps)

	metas := make([]appMeta, len(apps))

	for index := range apps {
		appName := names[index]

		// Skip surfaces that are never hint targets and steal ACTIVE right after a
		// libei cursor move.
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

// titleMissTTL is how long a title scan that found no window is trusted. It
// covers an app whose window gains its accessible title a moment after it
// opens.
const titleMissTTL = 10 * time.Second

// maxTitleMatches bounds each map of remembered title scans. An entry is a
// short key and value, and a session rarely runs more distinct apps than this,
// so a full map starts over rather than tracking which entry is oldest.
const maxTitleMatches = 64

// titleMiss is a title scan that read every window cleanly and found none
// with the title, and the applications on the bus when it ran.
type titleMiss struct {
	apps uint64
	at   time.Time
}

// findFocusedFrameByTitle finds the focused window by title, for a focused
// app_id that no AT-SPI application name answers to. GNOME apps name
// themselves after the binary ("gnome-text-editor") where the app_id is
// reverse-DNS ("org.gnome.TextEditor"). names are the ones the probe already
// read. It returns the frame, the path the logs name, and frames read.
//
// Reading every window costs tens of milliseconds on a busy desktop, so it
// first reads only the application that last matched this app_id, where the
// title must match too (a second instance of the app does not). A clean
// full-scan miss is remembered for titleMissTTL, so an app with no
// accessibility tree (foot) does not pay on every activation. An ambiguous or
// failed scan is not remembered.
func (c *Client) findFocusedFrameByTitle(
	ctx context.Context,
	conn *dbus.Conn,
	apps []accRef,
	names []string,
	focusedAppID, focusedTitle string,
) (accRef, string, int, bool) {
	if index, ok := c.rememberedTitleApp(focusedAppID, apps); ok {
		frame, scanned, found, _ := c.findFrameByTitle(
			ctx, conn, apps[index:index+1], names[index:index+1], focusedTitle)
		if found {
			return frame, "focused-title-remembered", scanned, true
		}
	}

	if c.titleScanMissed(focusedAppID, focusedTitle, apps) {
		return accRef{}, "", 0, false
	}

	frame, scanned, found, none := c.findFrameByTitle(ctx, conn, apps, names, focusedTitle)
	if found {
		c.rememberTitleMatch(focusedAppID, frame.Name)

		return frame, "focused-title-only", scanned, true
	}

	if none {
		c.rememberTitleMiss(focusedAppID, focusedTitle, apps)
	}

	c.logger.Debug("AT-SPI title scan found no window",
		zap.Int("apps", len(apps)),
		zap.Int("frames_scanned", scanned),
		zap.Bool("remembered", none))

	return accRef{}, "", scanned, false
}

// findFrameByTitle returns the one showing frame among apps titled
// focusedTitle, and how many frames it read. found reports a single match.
// none reports a clean scan with no match, the only result worth remembering,
// since a shared title or a failed read may resolve next time.
func (c *Client) findFrameByTitle(
	ctx context.Context,
	conn *dbus.Conn,
	apps []accRef,
	names []string,
	focusedTitle string,
) (accRef, int, bool, bool) {
	// A private diagnostic tells a clean miss from a failed one, and any failure
	// is passed on to the caller's.
	diag := &atspiScanDiag{}
	scanCtx := context.WithValue(ctx, atspiScanDiagKey{}, diag)

	metas := make([]appMeta, len(apps))
	for index, name := range names {
		metas[index] = appMeta{
			skip: isVirtualKeyboardApp(name) || isNonTargetSurfaceApp(name) ||
				isDesktopShellApp(name),
			matchesFocused: true,
		}
	}

	scans, scanned := c.scanApps(scanCtx, conn, apps, metas, true, focusedTitle, true)

	failure := diag.fatalErr()
	if failure != nil {
		c.noteCallErr(ctx, failure)
	}

	cand := mergeFrameScan(scans)
	if cand.focusedTitleCount == 1 {
		return cand.focusedTitleFrame, scanned, true, false
	}

	clean := failure == nil && ctx.Err() == nil

	return accRef{}, scanned, false, cand.focusedTitleCount == 0 && clean
}

// rememberTitleMatch records that the application on bus connection busName
// owned the window that matched focusedAppID by title.
func (c *Client) rememberTitleMatch(focusedAppID, busName string) {
	c.titleMatchedMu.Lock()
	defer c.titleMatchedMu.Unlock()

	if c.titleMatched == nil || len(c.titleMatched) >= maxTitleMatches {
		c.titleMatched = make(map[string]string)
	}

	c.titleMatched[focusedAppID] = busName
}

// rememberedTitleApp returns the index in apps of the application whose
// window last matched focusedAppID by title, if it is still on the bus.
func (c *Client) rememberedTitleApp(focusedAppID string, apps []accRef) (int, bool) {
	c.titleMatchedMu.Lock()
	busName, ok := c.titleMatched[focusedAppID]
	c.titleMatchedMu.Unlock()

	if !ok {
		return 0, false
	}

	for index, app := range apps {
		if app.Name == busName {
			return index, true
		}
	}

	return 0, false
}

// rememberTitleMiss records that no window among apps had focusedTitle.
func (c *Client) rememberTitleMiss(focusedAppID, focusedTitle string, apps []accRef) {
	c.titleMatchedMu.Lock()
	defer c.titleMatchedMu.Unlock()

	if c.titleMissed == nil || len(c.titleMissed) >= maxTitleMatches {
		c.titleMissed = make(map[string]titleMiss)
	}

	c.titleMissed[focusedAppID+"\x00"+focusedTitle] = titleMiss{
		apps: busFingerprint(apps),
		at:   time.Now(),
	}
}

// titleScanMissed reports whether a title scan for this app_id and title found
// no window within titleMissTTL, among the same applications.
func (c *Client) titleScanMissed(focusedAppID, focusedTitle string, apps []accRef) bool {
	c.titleMatchedMu.Lock()
	defer c.titleMatchedMu.Unlock()

	miss, ok := c.titleMissed[focusedAppID+"\x00"+focusedTitle]

	return ok && time.Since(miss.at) < titleMissTTL && miss.apps == busFingerprint(apps)
}

// busFingerprint hashes the bus names of apps, which change whenever an
// application starts or exits.
func busFingerprint(apps []accRef) uint64 {
	hash := fnv.New64a()
	for _, app := range apps {
		_, _ = hash.Write([]byte(app.Name))
		_, _ = hash.Write([]byte{0})
	}

	return hash.Sum64()
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

// appNames reads every application's Name concurrently over the multiplexed
// connection, so the probe costs the slowest reply rather than the sum. The
// semaphore keeps a busy desktop from flooding the a11y bus.
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

// findFocusedApps returns the indices of the applications whose name matches
// the focused app_id. It probes concurrently and cancels the rest on the
// first match, so a wedged background app cannot cost every selection
// atspiCallTimeout. Virtual keyboards, non-target surfaces and the shell never
// match. A no-match probe waits for every app and returns the names it read,
// which the title fallback reuses.
func (c *Client) findFocusedApps(
	ctx context.Context,
	conn *dbus.Conn,
	apps []accRef,
	focusedAppID string,
) ([]int, []string) {
	names := make([]string, len(apps))

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
				names[index] = name

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

	// Take the first match, then drain any already delivered. An app's windows
	// are frames within one AT-SPI app, so this is usually immediate. Draining
	// reports a focus shared by co-active apps as ambiguous. A match arriving
	// after the drain is missed, which needs two apps sharing an app_id with no
	// title to tell them apart.
	first, ok := <-matchCh
	if !ok {
		return matches, names
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

	// Stop probing so an unresponsive app never gates the result. The goroutine
	// closes matchCh once the canceled probes return.
	cancel()

	return matches, nil
}

// scanApps reads the selected applications' frames concurrently, returning
// one appFramesScan per app in registry order and the frames scanned.
// onlyMatching scans only the focused application(s), otherwise every
// non-skipped app. Each goroutine writes its own slot, so no lock is needed.
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

// scanOneApp reads an application's window frames, their ACTIVE/SHOWING
// state, and the titles of the focused app's showing frames. It only reads
// over the multiplexed connection, so it is safe for concurrent use.
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

// mergeFrameScan folds the per-app scans into frameCandidates in registry
// order. The shell is held aside, the focused app's showing and
// title-matching windows are counted, and the first ACTIVE/SHOWING frames
// become the cross-application fallbacks.
func mergeFrameScan(scans []appFramesScan) frameCandidates {
	var cand frameCandidates

	for _, scan := range scans {
		if !scan.scanned {
			continue
		}

		for _, frame := range scan.frames {
			// The desktop shell never wins the active-frame race. It is kept aside
			// and only used if no real application frame is found.
			if scan.isShell {
				if frame.showing && !cand.haveShell {
					cand.shellShowing = frame.ref
					cand.haveShell = true
				}

				continue
			}

			// Count the focused app's showing and title-matching windows so selectFrame
			// can tell a unique identification from an ambiguous one.
			if scan.matchesFocused && frame.showing {
				cand.focusedShowingCount++

				if cand.focusedShowingCount == 1 {
					cand.focusedShowingFrame = frame.ref
				}

				if frame.active {
					cand.focusedActiveCount++

					if cand.focusedActiveCount == 1 {
						cand.focusedActiveFrame = frame.ref
					}
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
		zap.Int("frames_scanned", framesScanned),
		zap.Duration("elapsed", time.Since(start)))
}

// frameCandidates holds the frames found during a findActiveFrame walk, ranked
// by how reliably each identifies the focused window, plus the compositor
// signals needed to choose between them.
type frameCandidates struct {
	// Frames of the focused application, with counts that expose ambiguity.
	focusedTitleFrame   accRef // first window whose title matched the focused toplevel
	focusedTitleCount   int    // windows of the focused app matching that title
	focusedShowingFrame accRef // first showing window of the focused app
	focusedShowingCount int    // showing windows of the focused app
	focusedActiveFrame  accRef // first ACTIVE showing window of the focused app
	focusedActiveCount  int    // ACTIVE showing windows of the focused app

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
	// focusedActiveDisambiguates is true where ACTIVE holds among one app's
	// windows but not across apps (X11). Ambiguous siblings resolve only to the
	// focused app's single ACTIVE window, never a guess or another app's frame.
	focusedActiveDisambiguates bool
}

// selectFrame chooses the focused frame from the ranked candidates. It is the
// pure decision half of findActiveFrame, split out so the ordering can be
// unit-tested without a live AT-SPI bus.
func selectFrame(cand frameCandidates) (accRef, bool) {
	switch {
	case cand.focusedTitleCount == 1:
		// Exactly one window of the focused app carries the focused toplevel's
		// title, an unambiguous match (works on any compositor).
		return cand.focusedTitleFrame, true
	case cand.focusedShowingCount == 1:
		// The focused app has a single showing window, so it is unambiguous even
		// without a title match.
		return cand.focusedShowingFrame, true
	case cand.haveFocused:
		// The focused app's window is not unique, because no frame matched or
		// siblings share or lack the title. KWin's ACTIVE still marks it (GTK
		// "Files" vs org.gnome.Nautilus, duplicate titles). On wlroots ACTIVE is
		// unreliable, so return nothing rather than guess.
		if cand.activeStateIdentifiesFocus && cand.haveActiveShowing {
			return cand.activeShowing, true
		}

		if cand.activeStateIdentifiesFocus && cand.haveActiveAny {
			return cand.activeAny, true
		}

		// X11: exactly one ACTIVE sibling of the focused app is that app's own
		// answer. Zero or several would leave enumeration order to choose, and
		// hints in the wrong sibling are worse than none.
		if cand.focusedActiveDisambiguates && cand.focusedActiveCount == 1 {
			return cand.focusedActiveFrame, true
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

// appMatchesFocusedID reports whether an AT-SPI application name corresponds
// to the focused app_id. AT-SPI reports a display name ("Firefox") where the
// app_id is often reverse-DNS ("org.mozilla.firefox"), so the
// case-insensitive match also compares the last dot-segment of either side.
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
