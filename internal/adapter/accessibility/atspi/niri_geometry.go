//go:build linux

package atspi

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/compositorcli"
)

// niri window-origin source. niri exposes the focused window's position within
// the workspace view (layout.tile_pos_in_workspace_view) plus the focused
// output's logical origin, which together give the window's screen origin.
// That field is populated for FLOATING windows; for tiled windows niri does not
// expose the on-screen position (upstream niri#2381), so originFor reports no
// origin and hints fall back to unoffset coordinates.
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

// originFor asks niri where the focused window is, in two queries with a gate
// between them.
//
// The gate is the tile position, which niri populates for floating windows only
// (niri#2381). Without it there is no origin to compute, so the second query is
// never made — and a focused-output query that would have failed cannot turn
// niri's ordinary layout into a reported failure on every activation.
func (n *niriOriginSource) originFor(frame windowFrame) (image.Point, bool, error) {
	var win niriWindow

	winErr := compositorcli.Query(&win, "niri", "msg", "-j", "focused-window")
	if winErr != nil {
		return image.Point{}, false, winErr
	}

	tileX, tileY, ok := niriOriginTile(win, frame.Width, frame.Height, n.logger)
	if !ok {
		return image.Point{}, false, nil
	}

	var out niriOutput

	outErr := compositorcli.Query(&out, "niri", "msg", "-j", "focused-output")
	if outErr != nil {
		return image.Point{}, false, outErr
	}

	return niriComputeOrigin(out, tileX, tileY), true, nil
}

// niriOriginTile returns the focused window's position within the workspace
// view, or false when niri has none to give: a tiled window
// (tile_pos_in_workspace_view absent — niri#2381), or a window whose size does
// not match the AT-SPI frame (a focus change raced the query).
//
// The pair is returned unpacked — x, then y, then whether there is one — rather
// than as niri's own slice, so the length check that makes it a pair cannot be
// separated from the values it guards.
func niriOriginTile(
	win niriWindow,
	frameW, frameH int,
	logger *zap.Logger,
) (float64, float64, bool) {
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

	return tile[0], tile[1], true
}

// niriComputeOrigin places a workspace-view tile position on screen by adding
// the focused output's logical origin.
func niriComputeOrigin(out niriOutput, tileX, tileY float64) image.Point {
	return image.Pt(out.Logical.X+int(tileX), out.Logical.Y+int(tileY))
}
