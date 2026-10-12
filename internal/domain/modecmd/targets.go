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

// targetsDocument is the JSON a rendering writes --targets back out as.
type targetsDocument struct {
	Hints []Target `json:"hints"`
}

// writtenTarget is a target as written, where a bound left out is nil rather
// than a zero that would read as a real coordinate.
type writtenTarget struct {
	X           *int   `json:"x"`
	Y           *int   `json:"y"`
	Width       *int   `json:"width"`
	Height      *int   `json:"height"`
	Role        string `json:"role"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Value       string `json:"value"`
}

// parseTargets reads --targets: at least one target, each with all four
// bounds and a positive width and height. Keys it does not know, such as the
// label a query reports, are ignored, so a query's answer reads back even
// after the query gains a key.
func parseTargets(value string) ([]Target, error) {
	var document struct {
		Hints []writtenTarget `json:"hints"`
	}

	err := json.Unmarshal([]byte(value), &document)
	if err != nil || len(document.Hints) == 0 {
		return nil, invalid(msgTargetsValue)
	}

	targets := make([]Target, 0, len(document.Hints))

	for _, written := range document.Hints {
		if written.X == nil || written.Y == nil || written.Width == nil || written.Height == nil ||
			*written.Width <= 0 || *written.Height <= 0 {
			return nil, invalid(msgTargetsValue)
		}

		targets = append(targets, Target{
			X:           *written.X,
			Y:           *written.Y,
			Width:       *written.Width,
			Height:      *written.Height,
			Role:        written.Role,
			Title:       written.Title,
			Description: written.Description,
			Value:       written.Value,
		})
	}

	return targets, nil
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
