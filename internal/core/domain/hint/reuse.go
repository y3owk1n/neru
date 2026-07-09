package hint

// LabelsByStableID maps each hint's element stable identity to its label.
// Elements without a stable identity are skipped. On a rare identity collision
// the first writer wins. Used to carry labels across an auto-refresh.
func LabelsByStableID(hints []*Interface) map[string]string {
	out := make(map[string]string, len(hints))

	for _, h := range hints {
		if h == nil || h.Element() == nil {
			continue
		}

		sid := h.Element().StableID()
		if sid == "" {
			continue
		}

		if _, exists := out[sid]; !exists {
			out[sid] = h.Label()
		}
	}

	return out
}

// ReuseLabels reassigns the labels of newHints so an element that persists from a
// previous scan (matched by stable identity) keeps its previous label, while
// preserving the exact set of labels the generator produced.
//
// The output is a bijection over the same label set the generator produced, so
// the prefix-free property of those labels is preserved by construction: labels
// are only permuted between elements, never invented or dropped.
//
// prevLabelByStable maps stable identity -> previous label (see LabelsByStableID).
// A hint with no stable identity, or whose previous label is not in the new label
// set (e.g. the set crossed a label-length boundary and the old label no longer
// exists), receives a fresh label from the remaining pool in the generator's
// order.
func ReuseLabels(newHints []*Interface, prevLabelByStable map[string]string) []*Interface {
	if len(newHints) == 0 || len(prevLabelByStable) == 0 {
		return newHints
	}

	available := make(map[string]bool, len(newHints))
	order := make([]string, len(newHints))

	for i, h := range newHints {
		available[h.Label()] = true
		order[i] = h.Label()
	}

	assigned := make([]string, len(newHints))
	taken := make(map[string]bool, len(newHints))

	// Pass 1: pin persistent elements to their previous label when it is still in
	// the new label set and not already claimed by another element.
	for i, h := range newHints {
		if h.Element() == nil {
			continue
		}

		sid := h.Element().StableID()
		if sid == "" {
			continue
		}

		old, ok := prevLabelByStable[sid]
		if !ok || !available[old] || taken[old] {
			continue
		}

		assigned[i] = old
		taken[old] = true
	}

	// Pass 2: the remaining (unclaimed) labels, in generator order, go to the
	// unpinned hints. The counts match exactly: one label is claimed per pin, so
	// remaining count == unpinned count.
	remaining := make([]string, 0, len(newHints))
	for _, label := range order {
		if !taken[label] {
			remaining = append(remaining, label)
		}
	}

	out := make([]*Interface, len(newHints))
	remainingIdx := 0

	for i, h := range newHints {
		label := assigned[i]
		if label == "" {
			label = remaining[remainingIdx]
			remainingIdx++
		}

		if label == h.Label() {
			out[i] = h

			continue
		}

		reassigned, err := NewHint(label, h.Element(), h.Position())
		if err != nil {
			out[i] = h

			continue
		}

		out[i] = reassigned
	}

	return out
}
