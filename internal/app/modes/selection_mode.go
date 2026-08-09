package modes

// defaultCursorFollowSelection is what a mode does with the real cursor while
// the user selects, when the activation said nothing about it: the cursor
// follows the selection.
//
// There is one default rather than one per mode because which modes have the
// axis at all is already stated twice, and neither statement is a switch:
// cursorFollowSelector (extensions.go) is the modes that carry it, and
// selectionModes (internal/domain/modecmd/flags.go) is the modes whose grammar
// accepts --cursor-selection-mode. Both name hints, grid and recursive grid,
// which are the only three modes that reach the function below — scroll and the
// monitor picker select nothing for a cursor to follow, so they never ask, and
// nothing else in the package calls it. A switch over domain.Mode
// here was a third copy of that list, with the three unreachable arms answering
// for modes that cannot reach it (internal/app/modes/AGENTS.md, the
// optional-extension contract).
const defaultCursorFollowSelection = true

// resolveCursorFollowSelection reads what an activation asked for, falling back
// to the default when it asked for nothing.
func resolveCursorFollowSelection(override *bool) bool {
	if override != nil {
		return *override
	}

	return defaultCursorFollowSelection
}
