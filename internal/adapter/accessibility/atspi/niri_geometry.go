//go:build linux

// niri window-origin source. niri exposes the focused window's position within
// the workspace view (layout.tile_pos_in_workspace_view) plus the focused
// output's logical origin, which together give the window's screen origin.
// That field is populated for FLOATING windows; for tiled windows niri does not
// expose the on-screen position (upstream niri#2381), so originFor reports no
// origin and hints fall back to unoffset coordinates.

package atspi

import (
	"go.uber.org/zap"
)

type niriOriginSource struct {
	logger *zap.Logger
}

func newNiriOriginSource(logger *zap.Logger) *niriOriginSource {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &niriOriginSource{logger: logger.Named("accessibility.niri")}
}

func (n *niriOriginSource) start() {}

// niriWindow mirrors the fields of `niri msg -j focused-window` we use.
type niriWindow struct {
	Layout struct {
		WindowSize             []int     `json:"window_size"`                //nolint:tagliatelle // niri wire format is snake_case.
		TilePosInWorkspaceView []float64 `json:"tile_pos_in_workspace_view"` //nolint:tagliatelle // niri wire format is snake_case.
	} `json:"layout"`
}

// niriOutput mirrors the fields of `niri msg -j focused-output` we use.
type niriOutput struct {
	Logical struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"logical"`
}

func (n *niriOriginSource) originFor(frameW, frameH int) (int, int, bool) {
	var win niriWindow
	if !compositorJSON(&win, "niri", "msg", "-j", "focused-window") {
		return 0, 0, false
	}

	var out niriOutput
	if !compositorJSON(&out, "niri", "msg", "-j", "focused-output") {
		return 0, 0, false
	}

	return niriComputeOrigin(win, out, frameW, frameH, n.logger)
}

// niriComputeOrigin derives the focused window's screen origin from niri's
// focused-window + focused-output data. It reports no origin for tiled windows
// (tile_pos_in_workspace_view absent — niri#2381) and when the window size does
// not match the AT-SPI frame (a focus change raced the query).
func niriComputeOrigin(
	win niriWindow,
	out niriOutput,
	frameW, frameH int,
	logger *zap.Logger,
) (int, int, bool) {
	tile := win.Layout.TilePosInWorkspaceView
	if len(tile) < coordPairLen {
		logger.Debug("niri origin unavailable: tiled window (niri#2381)")

		return 0, 0, false
	}

	if len(win.Layout.WindowSize) < coordPairLen ||
		absInt(win.Layout.WindowSize[0]-frameW) > windowOriginSizeTolerance ||
		absInt(win.Layout.WindowSize[1]-frameH) > windowOriginSizeTolerance {
		logger.Debug("niri origin rejected: window size does not match AT-SPI frame",
			zap.Ints("windowSize", win.Layout.WindowSize),
			zap.Int("frameW", frameW), zap.Int("frameH", frameH))

		return 0, 0, false
	}

	return out.Logical.X + int(tile[0]), out.Logical.Y + int(tile[1]), true
}
