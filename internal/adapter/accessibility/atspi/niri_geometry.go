//go:build linux

package atspi

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/compositorcli"
)

// niri window-origin source. The screen origin is the focused output's
// logical origin plus the window's tile position in the workspace view
// (layout.tile_pos_in_workspace_view) plus its offset inside the tile
// (layout.window_offset_in_tile, the border). niri sets the tile position for
// floating windows, and for tiled windows from niri#4711 on, the first release
// after 26.04. On an older niri a tiled window gets no origin (niri#2381).
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
		WindowOffsetInTile     []float64 `json:"window_offset_in_tile"`      //nolint:tagliatelle // niri wire format is snake_case.
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
// tile position. A tiled window on a niri before niri#4711 has none, so the
// output query is skipped and cannot turn niri's ordinary layout into a
// reported failure.
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

// niriOriginTile returns where the focused window's content starts in the
// workspace view, which is its tile position plus its offset inside the tile.
// It returns false when niri gave the window no tile position, as for a tiled
// window before niri#4711, or when the window's size does not match the AT-SPI
// frame because a focus change raced the query. It returns x, y and ok rather
// than niri's slices, so the length checks stay with the values they guard.
func niriOriginTile(
	win niriWindow,
	frameW, frameH int,
	logger *zap.Logger,
) (float64, float64, bool) {
	tile := win.Layout.TilePosInWorkspaceView
	if len(tile) < coordPairLen {
		logger.Debug("Niri origin unavailable: no tile position (tiled window before niri#4711)")

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

	offsetX, offsetY := niriOffsetInTile(win.Layout.WindowOffsetInTile)

	return tile[0] + offsetX, tile[1] + offsetY, true
}

// niriOffsetInTile returns where the window's content starts inside its tile,
// which is the border width when niri draws one. A reply without the field
// counts as no offset.
func niriOffsetInTile(offset []float64) (float64, float64) {
	if len(offset) < coordPairLen {
		return 0, 0
	}

	return offset[0], offset[1]
}

// niriComputeOrigin places a workspace-view position on screen by adding the
// focused output's logical origin.
func niriComputeOrigin(out niriOutput, tileX, tileY float64) image.Point {
	return image.Pt(out.Logical.X+int(tileX), out.Logical.Y+int(tileY))
}
