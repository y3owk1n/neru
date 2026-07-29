//go:build linux

//nolint:testpackage // Exercises the unexported per-compositor origin helpers directly.
package accessibility

import (
	"encoding/json"
	"testing"

	"go.uber.org/zap"
)

func TestNiriComputeOrigin(t *testing.T) {
	out := niriOutput{}
	out.Logical.X = 100
	out.Logical.Y = 10

	makeWindow := func(size []int, tile []float64) niriWindow {
		var w niriWindow

		w.Layout.WindowSize = size
		w.Layout.TilePosInWorkspaceView = tile

		return w
	}

	cases := []struct {
		name           string
		win            niriWindow
		frameW, frameH int
		wantX, wantY   int
		wantOK         bool
	}{
		{
			"floating window offset by output+tile",
			makeWindow([]int{946, 942}, []float64{958, 52}),
			946,
			942,
			1058,
			62,
			true,
		},
		{"tiled window has no tile_pos", makeWindow([]int{946, 942}, nil), 946, 942, 0, 0, false},
		{
			"size mismatch rejected",
			makeWindow([]int{946, 942}, []float64{958, 52}),
			500,
			942,
			0,
			0,
			false,
		},
		{
			"size within tolerance accepted",
			makeWindow([]int{946, 942}, []float64{958, 52}),
			950,
			940,
			1058,
			62,
			true,
		},
		{
			"missing window_size rejected",
			makeWindow(nil, []float64{958, 52}),
			946,
			942,
			0,
			0,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x, y, ok := niriComputeOrigin(tc.win, out, tc.frameW, tc.frameH, zap.NewNop())
			if ok != tc.wantOK || (ok && (x != tc.wantX || y != tc.wantY)) {
				t.Errorf("got (%d,%d,%v), want (%d,%d,%v)", x, y, ok, tc.wantX, tc.wantY, tc.wantOK)
			}
		})
	}
}

func TestHyprlandComputeOrigin(t *testing.T) {
	cases := []struct {
		name           string
		win            hyprlandWindow
		frameW, frameH int
		wantX, wantY   int
		wantOK         bool
	}{
		{
			"active window at/size",
			hyprlandWindow{At: []int{958, 52}, Size: []int{946, 942}},
			946,
			942,
			958,
			52,
			true,
		},
		{
			"size mismatch rejected",
			hyprlandWindow{At: []int{958, 52}, Size: []int{946, 942}},
			500,
			500,
			0,
			0,
			false,
		},
		{"missing at rejected", hyprlandWindow{Size: []int{946, 942}}, 946, 942, 0, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x, y, ok := hyprlandComputeOrigin(tc.win, tc.frameW, tc.frameH, zap.NewNop())
			if ok != tc.wantOK || (ok && (x != tc.wantX || y != tc.wantY)) {
				t.Errorf("got (%d,%d,%v), want (%d,%d,%v)", x, y, ok, tc.wantX, tc.wantY, tc.wantOK)
			}
		})
	}
}

// swayTreeJSON is a trimmed `swaymsg -t get_tree` with a focused window nested
// under output → workspace → container, plus a decoration offset in window_rect.
const swayTreeJSON = `{
  "focused": false, "rect": {"x":0,"y":0,"width":1920,"height":1080},
  "window_rect": {"x":0,"y":0,"width":0,"height":0},
  "nodes": [
    {"focused": false, "rect": {"x":960,"y":0,"width":960,"height":1080},
     "window_rect": {"x":0,"y":0,"width":0,"height":0},
     "nodes": [
       {"focused": true, "rect": {"x":960,"y":0,"width":960,"height":1080},
        "window_rect": {"x":2,"y":24,"width":956,"height":1052}, "nodes": [], "floating_nodes": []}
     ], "floating_nodes": []}
  ], "floating_nodes": []
}`

func TestSwayComputeOrigin(t *testing.T) {
	var tree swayNode

	err := json.Unmarshal([]byte(swayTreeJSON), &tree)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Content origin = rect(960,0) + window_rect(2,24) = (962,24); content size
	// = window_rect 956x1052.
	x, y, ok := swayComputeOrigin(&tree, 956, 1052, zap.NewNop())
	if !ok || x != 962 || y != 24 {
		t.Fatalf("focused content origin: got (%d,%d,%v), want (962,24,true)", x, y, ok)
	}

	// Size mismatch against the content size is rejected.
	if _, _, ok := swayComputeOrigin(&tree, 500, 500, zap.NewNop()); ok {
		t.Fatal("expected size mismatch to be rejected")
	}
}

func TestSwayFindFocusedNoneFocused(t *testing.T) {
	tree := swayNode{Focused: false, Nodes: []swayNode{{Focused: false}}}
	if findFocused(&tree) != nil {
		t.Fatal("expected no focused node")
	}
}
