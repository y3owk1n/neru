package modecmd

import "github.com/y3owk1n/neru/internal/domain/parity"

// PlatformSupport declares, for every mode flag, the platforms on which
// writing it does something.
//
// It sits beside the descriptor table rather than inside it because it answers
// a different question from the rest of a Descriptor: the descriptors say how a
// flag is written and read, and this says where writing it means anything.
// Keeping the two in one file would still leave a flag able to be declared
// without a column, which is what TestEveryModeFlagDeclaresItsPlatformSupport
// exists to prevent
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md).
//
// Every flag is everywhere today. The vision strategy value and --split-word
// carried a macOS-and-Linux column until Windows grew an OCR engine
// (Windows.Media.Ocr); add a narrow column back the moment a flag needs one
// rather than reaching for the nearest fit.
func PlatformSupport() parity.Declaration {
	return parity.Everywhere(parity.KindModeFlag,
		FlagAction.String(),
		FlagModifier.String(),
		FlagOnExit.String(),
		FlagRepeat.String(),
		FlagToggle.String(),
		FlagSearch.String(),
		FlagHideOnEmptySearch.String(),
		FlagRole.String(),
		FlagText.String(),
		FlagStrategy.String(),
		FlagSplitWord.String(),
		FlagCaptureScope.String(),
		FlagLabelDirection.String(),
		FlagZoomToDepth.String(),
		FlagCursorSelectionMode.String(),
	)
}
