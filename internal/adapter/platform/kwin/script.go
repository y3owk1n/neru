//go:build linux

package kwin

// The KWin script, and nothing else. It is JavaScript running inside the
// compositor rather than Go running here, and the rules it plays by are KWin's
// — so it is kept apart from the Go that receives what it sends.

// geometryScript is loaded into KWin. It reports the focused window's client
// geometry (content rect, excludes the titlebar so it aligns 1:1 with the
// AT-SPI content origin), and its identity, so a reader can tell which window
// the rectangle belongs to.
//
// It is driven by three families of signal, because activation alone describes
// only how a window's rectangle *starts*:
//
//   - activation (workspace.windowActivated) — which window to report on.
//   - geometry (clientGeometryChanged / frameGeometryChanged, and
//     interactiveMoveResizeFinished) — a window dragged, resized, tiled or
//     maximized without ever losing focus. Without these the cache is a
//     snapshot taken at the last focus change, and everything the user did to
//     the window since is reported at its old position.
//   - disappearance (workspace.windowRemoved, minimizedChanged, and activation
//     going to nothing) — without these the cache can never empty, so after
//     the last window closes the answer is still the dead window's rectangle
//     rather than "nothing is focused".
//
// The geometry handlers are connected once per window at windowAdded rather
// than connected and disconnected as focus moves: the handler is two property
// reads when the window is not the active one, and it never holds a reference
// to a window KWin is destroying. Windows already open when the script loads
// are picked up from workspace.stackingOrder.
//
// Two things are deliberately *not* reported. A drag is skipped while it is in
// progress (c.move / c.resize) because KWin emits a geometry signal per frame
// and each one would be a D-Bus message off the compositor thread; the final
// rectangle arrives on interactiveMoveResizeFinished. And a push identical to
// the last one is dropped, which covers the client/frame geometry signals
// arriving in pairs for one change.
//
// Three details are load-bearing and easy to undo by accident:
//
//   - **The desktop is tested before neruTransient, not after.** KWin defines
//     isSpecialWindow() as isDesktop() || isDock() || …, and KDE's desktop is a
//     plasmashell window — so the specialWindow flag and the class list below
//     would each swallow it, and the clear would never fire. Activating the
//     desktop is precisely how KDE says no application window is focused, which
//     is the opposite of what a transient surface says.
//   - **Every property reaching the payload goes through neruStr.** Absent KWin
//     properties are undefined, which is falsy and therefore safe in a guard —
//     but concatenating it yields the string "undefined", which the Go side
//     cannot tell from a window that really is called that. It would then read
//     as an identity that was reported and disagrees, and reject this window's
//     origin on every activation for as long as the session lasts.
//   - **A window that stops being reportable re-reads who is active** rather
//     than clearing outright (neruRefocus). Minimizing a background window must
//     not empty a cache that still describes the focused one.
//
// neruTransient filters out non-application surfaces that briefly take
// activation but are never hint targets: panels/docks/OSD/popups/tooltips/
// utility windows (caught by KWin's window-type flags), plus a few known
// transient classes (the XWayland video bridge, plasmashell, and the portal
// consent dialog). Focus landing on one of those keeps the last real window
// cached rather than clobbering it — without this, focus flicking to e.g.
// plasmashell or the RemoteDesktop consent dialog would mis-offset hint clicks.
//
// Accessing an absent KWin property yields undefined (falsy) and an absent
// signal yields undefined too, so the extra type flags and the guarded
// connects are both safe across KWin versions.
const geometryScript = `
function neruStr(v) {
    return (v === undefined || v === null) ? "" : "" + v;
}
function neruTransient(c) {
    if (neruStr(c.resourceClass) == "neru") return true;
    if (c.specialWindow || c.dock || c.splash ||
        c.utility || c.toolbar || c.menu || c.dropdownMenu || c.popupMenu ||
        c.tooltip || c.notification || c.criticalNotification ||
        c.onScreenDisplay || c.comboBox || c.dndIcon) return true;
    var cls = neruStr(c.resourceClass).toLowerCase();
    if (cls == "xwaylandvideobridge" || cls == "plasmashell" ||
        cls == "org.kde.plasmashell" ||
        cls == "org.freedesktop.impl.portal.desktop.kde") return true;
    return false;
}
var neruLast = "";
function neruCall(method, payload) {
    callDBus("org.neru.KWinBridge", "/org/neru/KWinBridge", "org.neru.KWinBridge",
             method, payload);
}
function neruClear(reason) {
    if (neruLast === "") return;
    neruLast = "";
    neruCall("ClearActiveWindow", reason);
}
function neruPush(c) {
    var g = c.clientGeometry ? c.clientGeometry : c.frameGeometry;
    if (!g) return;
    var payload = "" + Math.round(g.x) + "," + Math.round(g.y) + "," +
                  Math.round(g.width) + "," + Math.round(g.height) + "," +
                  neruStr(c.resourceClass) + "," + neruStr(c.resourceName) + "," +
                  neruStr(c.caption);
    if (payload === neruLast) return;
    neruLast = payload;
    neruCall("UpdateActiveWindow", payload);
}
function neruReportable(c) {
    return c.active && !c.desktopWindow && !neruTransient(c);
}
function neruActivated(c) {
    if (!c) { neruClear("unfocused"); return; }
    if (c.desktopWindow) { neruClear("desktop"); return; }
    if (neruTransient(c)) return;
    neruPush(c);
}
function neruRefocus(gone, reason) {
    var active = workspace.activeWindow;
    if (!active || active === gone) { neruClear(reason); return; }
    neruActivated(active);
}
function neruWatch(c) {
    if (!c) return;
    var onGeometry = function () {
        if (c.move || c.resize) return;
        if (!neruReportable(c)) return;
        neruPush(c);
    };
    if (c.clientGeometryChanged) c.clientGeometryChanged.connect(onGeometry);
    if (c.frameGeometryChanged) c.frameGeometryChanged.connect(onGeometry);
    if (c.interactiveMoveResizeFinished)
        c.interactiveMoveResizeFinished.connect(function () {
            if (neruReportable(c)) neruPush(c);
        });
    if (c.minimizedChanged)
        c.minimizedChanged.connect(function () {
            if (c.minimized) neruRefocus(c, "minimized");
        });
}
workspace.windowAdded.connect(neruWatch);
workspace.windowActivated.connect(neruActivated);
workspace.windowRemoved.connect(function (c) { neruRefocus(c, "closed"); });
var neruOpen = workspace.stackingOrder;
for (var neruI = 0; neruI < neruOpen.length; neruI++) neruWatch(neruOpen[neruI]);
neruActivated(workspace.activeWindow);
`
