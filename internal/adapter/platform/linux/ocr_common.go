//go:build linux

package linux

import (
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
)

// ocrLanguage is the language tesseract is initialized for.
//
// It is a constant rather than an option, and that is the "every option is a
// cost" rule rather than an oversight: macOS's Vision framework takes no
// language from the user either, so exposing one here would be a Linux-only
// word with no counterpart to have parity with. Latin-script UI text recognizes
// acceptably under eng regardless of the interface language, and a user who
// needs another model points TESSDATA_PREFIX at it — the resolution below reads
// the environment first for exactly that reason.
const ocrLanguage = "eng"

// ocrLanguageData is the file whose absence means "the vision strategy cannot
// run here", and the name every such error carries so a user knows what to
// install.
const ocrLanguageData = ocrLanguage + ".traineddata"

// tessdataEnvVar is where a user or a Nix wrapper says which tessdata to use.
const tessdataEnvVar = "TESSDATA_PREFIX"

// tessdataSystemDirs are the directories distributions install language data
// into, in the order they are tried. They differ per distribution and per
// tesseract major version, which is why this is a list rather than a path:
// Debian and Ubuntu version the directory, Fedora does not, Arch uses the
// unversioned share path, and a hand-built tesseract lands in /usr/local.
var tessdataSystemDirs = []string{
	"/usr/share/tessdata",
	tessdataDebianDir,
	"/usr/share/tesseract-ocr/4.00/tessdata",
	"/usr/share/tesseract-ocr/tessdata",
	"/usr/share/tesseract/tessdata",
	"/usr/local/share/tessdata",
}

// tessdataDebianDir is where Debian and Ubuntu put tesseract 5's language
// data. It is named because it is the one the tests reach for: it is the layout
// that shows both halves of the resolution — a versioned directory, and a
// TESSDATA_PREFIX that may point at it or at its parent.
const tessdataDebianDir = "/usr/share/tesseract-ocr/5/tessdata"

// ocrStatus mirrors the NERU_OCR_* codes in ocr.h, for the reason captureStatus
// mirrors the capture ones: a Go test file cannot use cgo, and the mapping from
// a native failure to the sentence a user reads is the part worth testing.
// ocr_cgo.go carries constant expressions that fail to compile if the two lists
// disagree.
type ocrStatus int

const (
	ocrStatusOK ocrStatus = iota
	ocrStatusInit
	ocrStatusAlloc
	ocrStatusImage
	ocrStatusRecognize
)

// OCRWord is one run of recognized text, in the coordinate space of the image
// it was read from — origin (0, 0) at the top-left of that buffer, not of the
// screen.
//
// Text is screen content and carries the same rules the capture buffer does: it
// is never logged, never written to disk, and never held past the detection
// that asked for it.
type OCRWord struct {
	Text   string
	Bounds image.Rectangle
	// Confidence is 0..1. Tesseract reports 0..100; the native bridge
	// normalizes so this is the same scale hints.vision.minimum_confidence and
	// the macOS backend use.
	Confidence float64
}

// OCRParams is what one recognition needs beyond the pixels.
type OCRParams struct {
	// WordLevel asks for per-word boxes instead of per-line ones, which is
	// what `neru hints --split-word` means.
	WordLevel bool
	// TimeoutMS bounds the recognition. Zero or less means no deadline.
	TimeoutMS int
}

// ocrError maps a native OCR status onto the shared error vocabulary.
//
// Only initialization is CodeNotSupported: it means this machine cannot run the
// engine, so a caller should degrade rather than retry. Everything else is a
// live failure of one recognition.
func ocrError(status ocrStatus) error {
	switch status {
	case ocrStatusInit:
		return derrors.New(
			derrors.CodeNotSupported,
			"the OCR engine could not be initialized: the tesseract language data "+
				"was found but could not be loaded; reinstall the "+ocrLanguageData+" package",
		)
	case ocrStatusAlloc:
		return derrors.New(
			derrors.CodeInternal,
			"could not allocate memory for the text recognition result",
		)
	case ocrStatusImage:
		return derrors.New(
			derrors.CodeActionFailed,
			"the captured frame is not a shape the OCR engine can read",
		)
	// ocrStatusOK rides with the recognition failure rather than getting its
	// own sentence, the way captureError pairs captureStatusOK with
	// captureStatusFailed: this function is only reached on a failure branch,
	// so being handed the success code means the caller's branch is wrong, and
	// answering nil would turn that into a silent success.
	case ocrStatusOK, ocrStatusRecognize:
		return derrors.New(
			derrors.CodeActionFailed,
			"text recognition failed or ran past hints.vision.request_timeout_ms",
		)
	default:
		return derrors.New(
			derrors.CodeActionFailed,
			"text recognition failed for an unknown reason",
		)
	}
}

// resolveTessdataDir finds the directory holding the language data, or reports
// CodeNotSupported naming the file that is absent.
//
// The environment wins over the system paths, because it is the only answer a
// user can give: a Nix store path, a tessdata_fast checkout, or a language pack
// installed somewhere a distribution never puts one. Both readings of
// TESSDATA_PREFIX are accepted — tesseract 4 documented it as the parent of a
// tessdata/ directory and tesseract 5 as the directory itself, and both
// spellings are live in shell profiles today.
//
// env and exists are parameters so this is testable without a tesseract
// installation, which is also the state most CI runners are in.
func resolveTessdataDir(env func(string) string, exists func(string) bool) (string, error) {
	var candidates []string

	if prefix := strings.TrimSpace(env(tessdataEnvVar)); prefix != "" {
		candidates = append(candidates, prefix, filepath.Join(prefix, "tessdata"))
	}

	candidates = append(candidates, tessdataSystemDirs...)

	for _, dir := range candidates {
		if exists(filepath.Join(dir, ocrLanguageData)) {
			return dir, nil
		}
	}

	return "", derrors.Newf(
		derrors.CodeNotSupported,
		"the vision hint strategy needs tesseract language data, and %s was not found "+
			"in %s or any of %s; install it (tesseract-ocr-eng on Debian and Ubuntu, "+
			"tesseract-langpack-eng on Fedora, tesseract-data-eng on Arch) or point %s at it",
		ocrLanguageData,
		tessdataEnvVar,
		strings.Join(tessdataSystemDirs, ", "),
		tessdataEnvVar,
	)
}

// tessdataDir resolves the language data directory against the real
// environment and filesystem.
func tessdataDir() (string, error) {
	return resolveTessdataDir(os.Getenv, func(path string) bool {
		info, err := os.Stat(path)

		return err == nil && !info.IsDir()
	})
}
