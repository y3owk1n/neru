package modecmd

import "github.com/y3owk1n/neru/internal/domain/parity"

// Why a mode flag's platform column is narrower than every platform.
const (
	noteVisionStrategy = "the vision strategy needs an element-detection engine, which " +
		"macOS has in the Vision framework and Linux in tesseract; Windows has neither, " +
		"so detection returns nothing and no hints appear; use axtree"
	noteSplitWord = "splitting detected text into words needs the vision strategy, " +
		"which Windows has no engine for; there the flag is refused rather than ignored"
)

// visionStrategy is the --strategy value that selects the vision engine. The
// flag itself is recognized everywhere; this one value of it is not. The
// contour value carries no declaration: every platform captures the screen it
// runs over.
const visionStrategy = "vision"

// darwinAndLinux is the column the vision words carry: both platforms have an
// element-detection engine behind the strategy, and Windows has none. Named
// rather than written out at each site so a reader compares the two by the same
// words, as config's declaration does.
var darwinAndLinux = parity.Platforms{parity.Darwin, parity.Linux}

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
func PlatformSupport() parity.Declaration {
	return parity.Join(
		parity.On(parity.KindModeFlag, darwinAndLinux, noteSplitWord,
			FlagSplitWord.String(),
		),
		parity.ValueOn(parity.KindModeFlag, darwinAndLinux, noteVisionStrategy,
			visionStrategy,
			FlagStrategy.String(),
		),

		parity.Everywhere(parity.KindModeFlag,
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
			FlagCaptureScope.String(),
			FlagLabelDirection.String(),
			FlagZoomToDepth.String(),
			FlagCursorSelectionMode.String(),
		),
	)
}
