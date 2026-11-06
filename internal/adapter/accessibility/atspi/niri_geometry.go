//go:build linux

package atspi

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/compositorcli"
)

// niri window-origin source. The focused window's position in the workspace
// view (layout.tile_pos_in_workspace_view) plus the focused output's logical
// origin give its screen origin. niri sets that field only for floating
// windows (niri#2381), so a tiled window gets no origin.
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

// originFor asks niri where the focused window is, in two queries gated on the
// tile position. A tiled window has none (niri#2381), so the output query is
// skipped and cannot turn niri's ordinary layout into a reported failure.
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

// niriOriginTile returns the focused window's workspace-view position, or
// false for a tiled window (niri#2381) or one whose size does not match the
// AT-SPI frame (a focus change raced the query). It returns x, y and ok rather
// than niri's slice, so the length check stays with the values it guards.
func niriOriginTile(
	win niriWindow,
	frameW, frameH int,
	logger *zap.Logger,
) (float64, float64, bool) {
	tile := win.Layout.TilePosInWorkspaceView
	if len(tile) < coordPairLen {
		logger.Debug("Niri origin unavailable: tiled window (niri#2381)")

		return 0, 0, false
	}

	if len(win.Layout.WindowSize) < coordPairLen ||
		absInt(win.Layout.WindowSize[0]-frameW) > windowOriginSizeTolerance ||
		absInt(win.Layout.WindowSize[1]-frameH) > windowOriginSizeTolerance {
		logger.Debug("Niri origin rejected: window size does not match AT-SPI frame",
			zap.Ints("window_size", win.Layout.WindowSize),
			zap.Int("frame_w", frameW), zap.Int("frame_h", frameH))

		return 0, 0, false
	}

	return tile[0], tile[1], true
}

// niriComputeOrigin places a workspace-view tile position on screen by adding
// the focused output's logical origin.
func niriComputeOrigin(out niriOutput, tileX, tileY float64) image.Point {
	return image.Pt(out.Logical.X+int(tileX), out.Logical.Y+int(tileY))
}
