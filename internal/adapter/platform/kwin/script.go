//go:build linux

package kwin

// The KWin script, kept apart from the Go that receives what it sends.

// geometryScript is loaded into KWin. It reports the focused window's client
// geometry (content rect, excluding the titlebar so it aligns 1:1 with the
// AT-SPI content origin) and its identity.
//
// It is driven by three families of signal:
//
//   - activation (workspace.windowActivated) says which window to report on.
//   - geometry (clientGeometryChanged / frameGeometryChanged, and
//     interactiveMoveResizeFinished) covers a window moved, resized, tiled or
//     maximized without losing focus.
//   - disappearance (workspace.windowRemoved, minimizedChanged, and activation
//     going to nothing) lets the cache empty, so a closed window's rectangle
//     is not reported after it is gone.
//
// Geometry handlers connect once per window at windowAdded, and windows open
// at load are picked up from workspace.stackingOrder. The handler is cheap for
// inactive windows and never holds a reference to a window being destroyed.
//
// A drag in progress (c.move / c.resize) is not reported, because KWin emits a
// geometry signal per frame and each would be a D-Bus message off the
// compositor thread. The final rectangle arrives on
// interactiveMoveResizeFinished. A push identical to the last is dropped,
// which covers client/frame signals arriving in pairs.
//
// Three details are load-bearing and easy to undo by accident:
//
//   - **The desktop is tested before neruTransient, not after.** KDE's desktop
//     is a plasmashell window and isSpecialWindow() includes isDesktop(), so
//     neruTransient would swallow it and the clear would never fire.
//   - **Every property reaching the payload goes through neruStr.** An absent
//     KWin property concatenates as "undefined", which the Go side would read
//     as a disagreeing identity and reject this window's origin all session.
//   - **A window that stops being reportable re-reads who is active** rather
//     than clearing outright (neruRefocus). Minimizing a background window must
//     not empty a cache that still describes the focused one.
//
// neruTransient filters surfaces that briefly take activation but are never
// hint targets: KWin's non-application window types plus a few transient
// classes (the XWayland video bridge, plasmashell, the portal consent dialog).
// Focus landing on one keeps the last real window cached, or hint clicks would
// mis-offset. Absent KWin properties and signals are undefined, so the extra
// type flags and guarded connects are safe across KWin versions.
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
