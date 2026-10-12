package modecmd

import (
	"encoding/json"
)

// Target is one place a caller supplies for hints mode to label, in place of
// the elements a scan would find. Its fields and keys are the ones `neru
// query hints` reports, so a query's answer can be filtered and handed back.
// Only the bounds are required, in the coordinates move_mouse takes.
type Target struct {
	X           int    `json:"x"`
	Y           int    `json:"y"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Role        string `json:"role,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value,omitempty"`
}

// targetsDocument is the JSON --targets carries. Keys it does not know, such
// as the label a query reports, are ignored, so a query's answer reads back
// even after the query gains a key.
type targetsDocument struct {
	Hints []Target `json:"hints"`
}

// parseTargets reads --targets: at least one target, each with a width and a
// height.
func parseTargets(value string) ([]Target, error) {
	var document targetsDocument

	err := json.Unmarshal([]byte(value), &document)
	if err != nil || len(document.Hints) == 0 {
		return nil, invalid(msgTargetsValue)
	}

	for _, target := range document.Hints {
		if target.Width <= 0 || target.Height <= 0 {
			return nil, invalid(msgTargetsValue)
		}
	}

	return document.Hints, nil
}

// renderTargets writes the targets back out as the JSON they were read from.
func renderTargets(activation Activation) []string {
	if activation.Targets == nil {
		return nil
	}

	encoded, _ := json.Marshal( //nolint:errchkjson // ints and strings always encode
		targetsDocument{Hints: activation.Targets},
	)

	return []string{FlagTargets.Assign(string(encoded))}
}
