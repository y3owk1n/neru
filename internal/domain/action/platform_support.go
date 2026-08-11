package action

import "github.com/y3owk1n/neru/internal/domain/parity"

// Why an action's platform column is narrower than every platform.
const (
	noteCursorVisibility = "a Wayland client may not hide another client's cursor, " +
		"and the blessed Linux stack is Wayland; Windows has no equivalent either"
	noteHorizontalScroll = "the Windows wheel event carries no horizontal delta, " +
		"so a sideways scroll injects nothing"
	noteKeyFeed = "Windows has no key-injection path yet, so the key it would post is " +
		"never sent; the key_feed capability reports stub to match"
)

// PlatformSupport declares, for every action name, the platforms on which
// writing it in a binding does something.
//
// It is exhaustive: an action supported everywhere says so rather than
// inheriting it by omission, which is what lets
// TestEveryActionDeclaresItsPlatformSupport fail the build on an action added
// without the decision being made
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md).
func PlatformSupport() parity.Declaration {
	return parity.Join(
		parity.On(parity.KindAction, parity.Platforms{parity.Darwin}, noteCursorVisibility,
			string(NameHideCursor),
			string(NameShowCursor),
		),

		parity.On(
			parity.KindAction,
			parity.Platforms{parity.Darwin, parity.Linux},
			noteHorizontalScroll,
			string(NameScrollLeft),
			string(NameScrollRight),
		),

		parity.On(
			parity.KindAction,
			parity.Platforms{parity.Darwin, parity.Linux},
			noteKeyFeed,
			string(NameFeed),
		),

		parity.Everywhere(parity.KindAction,
			string(NameLeftClick),
			string(NameRightClick),
			string(NameMiddleClick),
			string(NameLeftMouseDown),
			string(NameLeftMouseUp),
			string(NameMouseDown),
			string(NameMouseUp),
			string(NameRightMouseDown),
			string(NameRightMouseUp),
			string(NameMiddleMouseDown),
			string(NameMiddleMouseUp),
			string(NameLeftMouseToggle),
			string(NameRightMouseToggle),
			string(NameMiddleMouseToggle),
			string(NameMoveMouse),
			string(NameMoveMouseRelative),
			string(NameScroll),
			string(NameReset),
			string(NameBackspace),
			string(NameMoveCell),
			string(NameWaitForModeExit),
			string(NameSaveCursorPos),
			string(NameRestoreCursorPos),
			string(NameScrollUp),
			string(NameScrollDown),
			string(NameMoveMonitor),
			string(NameGoTop),
			string(NameGoBottom),
			string(NamePageUp),
			string(NamePageDown),
			string(NameSleep),
			string(NameCycleHint),
			string(NameSearchHints),
		),
	)
}
