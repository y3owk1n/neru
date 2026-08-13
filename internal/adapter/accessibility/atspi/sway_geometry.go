//go:build linux

package atspi

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/compositorcli"
)

// Sway window-origin source. `swaymsg -t get_tree` returns the full node tree;
// the focused node's on-screen content origin is rect + window_rect (window_rect
// is the content area relative to rect, i.e. it excludes server-side
// decorations so it aligns with the AT-SPI content origin).
type swayOriginSource struct {
	logger *zap.Logger
}

func newSwayOriginSource(logger *zap.Logger) *swayOriginSource {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &swayOriginSource{logger: logger.Named("accessibility.sway")}
}

func (s *swayOriginSource) start() {}

type swayRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// swayNode mirrors the fields of `swaymsg -t get_tree` we use. rect is the
// node's absolute geometry; window_rect is the content area relative to rect.
type swayNode struct {
	Focused       bool       `json:"focused"`
	Rect          swayRect   `json:"rect"`
	WindowRect    swayRect   `json:"window_rect"` //nolint:tagliatelle // sway wire format is snake_case.
	Nodes         []swayNode `json:"nodes"`
	FloatingNodes []swayNode `json:"floating_nodes"` //nolint:tagliatelle // sway wire format is snake_case.
}

// findFocused returns the focused node in the tree, if any.
func findFocused(node *swayNode) *swayNode {
	if node.Focused {
		return node
	}

	for i := range node.Nodes {
		if hit := findFocused(&node.Nodes[i]); hit != nil {
			return hit
		}
	}

	for i := range node.FloatingNodes {
		if hit := findFocused(&node.FloatingNodes[i]); hit != nil {
			return hit
		}
	}

	return nil
}

func (s *swayOriginSource) originFor(frame windowFrame) (image.Point, bool, error) {
	var tree swayNode

	err := compositorcli.Query(&tree, "swaymsg", "-t", "get_tree")
	if err != nil {
		return image.Point{}, false, err
	}

	origin, ok := swayComputeOrigin(&tree, frame.Width, frame.Height, s.logger)

	return origin, ok, nil
}

// swayComputeOrigin finds the focused node in a sway tree and derives its
// on-screen content origin (rect + window_rect, which excludes decorations),
// rejecting a size mismatch with the AT-SPI frame.
func swayComputeOrigin(
	tree *swayNode,
	frameW, frameH int,
	logger *zap.Logger,
) (image.Point, bool) {
	focused := findFocused(tree)
	if focused == nil {
		return image.Point{}, false
	}

	// Content origin/size: prefer window_rect (excludes decorations); fall back
	// to rect when window_rect carries no size.
	contentW, contentH := focused.WindowRect.Width, focused.WindowRect.Height
	originX := focused.Rect.X + focused.WindowRect.X
	originY := focused.Rect.Y + focused.WindowRect.Y

	if contentW <= 0 || contentH <= 0 {
		contentW, contentH = focused.Rect.Width, focused.Rect.Height
		originX, originY = focused.Rect.X, focused.Rect.Y
	}

	if absInt(contentW-frameW) > windowOriginSizeTolerance ||
		absInt(contentH-frameH) > windowOriginSizeTolerance {
		logger.Debug("sway origin rejected: window size does not match AT-SPI frame",
			zap.Int("contentW", contentW), zap.Int("contentH", contentH),
			zap.Int("frameW", frameW), zap.Int("frameH", frameH))

		return image.Point{}, false
	}

	return image.Pt(originX, originY), true
}
