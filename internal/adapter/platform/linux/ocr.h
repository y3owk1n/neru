#ifndef OCR_H
#define OCR_H

// Tesseract-backed text recognition for the Linux half of ports.VisionPort,
// turning screencapture.h pixels into positioned words. macOS answers the port
// with three Vision requests, and OCR answers only the text one, so Linux
// `vision` is text-only and the rectangle options are declared macOS-only
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md).
//
// Privacy: recognized text is screen content. Nothing here writes it anywhere
// but the result. neru_ocr_result_free wipes the strings,
// `tessedit_write_images` and `debug_file` are pinned so tesseract writes
// neither an image nor a log, and every recognition ends with TessBaseAPIClear.

#define NERU_OCR_OK 0
// The engine could not be initialized for the given tessdata directory and
// language. In practice that means language data that is absent or unreadable.
#define NERU_OCR_ERR_INIT 1
// Out of memory building the result.
#define NERU_OCR_ERR_ALLOC 2
// The caller passed no pixels, or a degenerate image.
#define NERU_OCR_ERR_IMAGE 3
// Recognition failed, or ran past the caller's deadline.
#define NERU_OCR_ERR_RECOGNIZE 4
// The engine was busy with another recognition for longer than this caller was
// willing to wait, or could not be created yet because the X11 hint-search
// field held the locale guard (locale_guard.h).
#define NERU_OCR_ERR_BUSY 5
// Recognition ran past the caller's deadline. Split from ERR_RECOGNIZE because
// a timeout wants a bigger budget or smaller region, and a failure a bad frame.
#define NERU_OCR_ERR_TIMEOUT 6

typedef struct {
	// text is a NUL-terminated UTF-8 string owned by the result.
	char *text;
	// Bounding box in the captured image's own pixel space. Origin (0, 0) is
	// the top-left of the buffer that was handed in, not of the screen.
	int x;
	int y;
	int width;
	int height;
	// confidence is 0..1. Tesseract reports 0..100 and it is normalized here,
	// so the Go side compares against hints.vision.minimum_confidence on the
	// same scale the macOS backend uses.
	float confidence;
} NeruOCRWord;

typedef struct {
	NeruOCRWord *words;
	int count;
	// How long recognition took, in ms, filled on failure too, because "took
	// the whole budget" and "gave up at once" are different bugs. A duration
	// describes the work, not the screen, so it is loggable.
	int elapsedMS;
} NeruOCRResult;

typedef struct {
	// datapath is the directory holding <language>.traineddata. Never NULL.
	// Resolution is done in Go so a missing tessdata reports which file is
	// absent rather than failing inside tesseract.
	const char *datapath;
	const char *language;
	// wordLevel selects per-word boxes (1) over per-line boxes (0), which is
	// what `neru hints --split-word` asks for.
	int wordLevel;
	// timeoutMS bounds one recognition. <= 0 means no deadline.
	int timeoutMS;
} NeruOCRConfig;

// One character of a line read, with the ink box tesseract placed it in. line
// and word count from 0 in reading order, so the characters of a word share a
// word number within their line.
typedef struct {
	// text is the character as NUL-terminated UTF-8, owned by the result.
	char *text;
	// Bounding box of its ink in the captured image's own pixel space.
	int x;
	int y;
	int width;
	int height;
	int line;
	int word;
} NeruOCRSymbol;

typedef struct {
	NeruOCRSymbol *symbols;
	int count;
	// How long recognition took, in ms, as NeruOCRResult has it.
	int elapsedMS;
} NeruOCRLines;

// neru_ocr_probe reports whether the engine can be initialized for this
// datapath and language, warming it in the process. NERU_OCR_OK or
// NERU_OCR_ERR_INIT.
int neru_ocr_probe(const NeruOCRConfig *config);

// neru_ocr_recognize reads RGBA8888 pixels and fills out with the words found.
// stride is the byte distance between rows. A frame with no text is
// NERU_OCR_OK with a zero count, not an error.
int neru_ocr_recognize(
    const unsigned char *pixels, int width, int height, int stride, const NeruOCRConfig *config, NeruOCRResult *out);

// neru_ocr_result_free wipes and releases a result. Safe on a zeroed struct.
void neru_ocr_result_free(NeruOCRResult *result);

// neru_ocr_recognize_lines reads RGBA8888 pixels as neru_ocr_recognize does
// and fills out with every character found, with its line and word. It is for
// a caller that lays the text out as the screen shows it. config->wordLevel
// is ignored.
int neru_ocr_recognize_lines(
    const unsigned char *pixels, int width, int height, int stride, const NeruOCRConfig *config, NeruOCRLines *out);

// neru_ocr_lines_free wipes and releases what neru_ocr_recognize_lines filled.
// Safe on a zeroed struct.
void neru_ocr_lines_free(NeruOCRLines *lines);

#endif /* OCR_H */
