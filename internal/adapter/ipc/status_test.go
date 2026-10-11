package ipc_test

import (
	"encoding/json"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
)

// TestStatusData_EncodesAsTheSortedMapItReplaced pins `neru status --json` and
// the watch snapshot to the bytes they had as a map, whose keys the encoder
// sorts. A field declared out of key order would reorder the output.
func TestStatusData_EncodesAsTheSortedMapItReplaced(t *testing.T) {
	following := true

	status := ipc.StatusData{
		Enabled:               true,
		Mode:                  "hints",
		Config:                "/tmp/config.toml",
		Capabilities:          map[string]any{"platform": "darwin"},
		Profile:               map[string]any{"os": "darwin"},
		CursorFollowSelection: &following,
		SavedCursorSlots:      map[string]ipc.CursorSlotData{"home": {X: 1, Y: 2}},
		StickyModifiers:       "cmd",
	}

	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var asMap map[string]any

	err = json.Unmarshal(encoded, &asMap)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	sorted, err := json.Marshal(asMap)
	if err != nil {
		t.Fatalf("json.Marshal(map) error = %v", err)
	}

	if string(encoded) != string(sorted) {
		t.Errorf("status encodes as\n%s\nwant the sorted\n%s", encoded, sorted)
	}
}
