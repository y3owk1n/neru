//go:build linux

package linux

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
)

// ocrLanguage is the language tesseract is initialized for. It is a constant,
// not an option, by the "every option is a cost" rule. macOS Vision takes no
// language either, Latin-script UI text reads acceptably under eng, and a user
// who needs another model points TESSDATA_PREFIX at it, which is why the
// resolution below reads the environment first.
const ocrLanguage = "eng"

// ocrLanguageData is the file whose absence means "the vision strategy cannot
// run here", and the name every such error carries so a user knows what to
// install.
const ocrLanguageData = ocrLanguage + ".traineddata"

// tessdataEnvVar is where a user or a Nix wrapper says which tessdata to use.
const tessdataEnvVar = "TESSDATA_PREFIX"

// tessdataSystemDirs are where distributions install language data, in try
// order. The path differs per distribution and per tesseract major version.
var tessdataSystemDirs = []string{
	"/usr/share/tessdata",
	tessdataDebianDir,
	"/usr/share/tesseract-ocr/4.00/tessdata",
	"/usr/share/tesseract-ocr/tessdata",
	"/usr/share/tesseract/tessdata",
	"/usr/local/share/tessdata",
}

// tessdataDebianDir is where Debian and Ubuntu put tesseract 5's language
// data. It is named because the tests use it.
const tessdataDebianDir = "/usr/share/tesseract-ocr/5/tessdata"

// ocrStatus mirrors the NERU_OCR_* codes in ocr.h so the status-to-sentence
// mapping is testable without cgo. ocr_cgo.go fails to compile if the two
// lists disagree.
type ocrStatus int

const (
	ocrStatusOK ocrStatus = iota
	ocrStatusInit
	ocrStatusAlloc
	ocrStatusImage
	ocrStatusRecognize
	ocrStatusBusy
	ocrStatusTimeout
)

// OCRWord is one run of recognized text, with Bounds relative to the top-left
// of the image it was read from, not the screen. Text is screen content. Never
// log it, write it to disk, or hold it past the detection that asked for it.
type OCRWord struct {
	Text   string
	Bounds image.Rectangle
	// Confidence is 0..1, normalized from tesseract's 0..100 to match
	// hints.vision.minimum_confidence and the macOS backend.
	Confidence float64
}

// OCRStats describes the work one recognition did, in durations and counts
// only, so a caller can log it.
type OCRStats struct {
	// Recognition is how long the engine spent on the frame, which is the
	// number that separates "gave up on the deadline" from "failed instantly".
	Recognition time.Duration
}

// OCRParams is what one recognition needs beyond the pixels.
type OCRParams struct {
	// WordLevel asks for per-word boxes instead of per-line ones, which is
	// what `neru hints --split-word` means.
	WordLevel bool
	// TimeoutMS bounds the recognition. Zero or less means no deadline.
	TimeoutMS int
}

// ocrError maps a native OCR status onto the shared error vocabulary. Only
// initialization is CodeNotSupported, meaning this machine cannot run the
// engine and callers should degrade. Everything else is one failed recognition.
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
	case ocrStatusBusy:
		return derrors.New(
			derrors.CodeActionFailed,
			"the OCR engine was busy with an earlier frame or behind an input method "+
				"opening for hint search, so try again",
		)
	case ocrStatusTimeout:
		return derrors.New(
			derrors.CodeActionFailed,
			"text recognition ran past hints.vision.request_timeout_ms; raise it, or "+
				"narrow what is being read by hinting a smaller window",
		)
	// ocrStatusOK only reaches here from a wrong failure branch. Answering nil
	// would make that a silent success, so it shares the recognition failure,
	// as captureError does with captureStatusOK.
	case ocrStatusOK, ocrStatusRecognize:
		return derrors.New(
			derrors.CodeActionFailed,
			"the OCR engine could not read the captured frame",
		)
	default:
		return derrors.New(
			derrors.CodeActionFailed,
			"text recognition failed for an unknown reason",
		)
	}
}

// resolveTessdataDir finds the directory holding the language data, or reports
// CodeNotSupported naming the absent file. The environment wins over system
// paths because it is the only answer a user can give, such as a Nix store
// path. TESSDATA_PREFIX is accepted as the tessdata directory (tesseract 5) or
// its parent (tesseract 4), since both are live in shell profiles. env and
// exists are parameters so tests need no tesseract installation.
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
